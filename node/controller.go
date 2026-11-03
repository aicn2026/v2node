package node

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/common/task"
	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core"
	"github.com/wyx2685/v2node/limiter"
)

type Controller struct {
	server                  *core.V2Core
	apiClient               *panel.Client
	tag                     string
	limiter                 *limiter.Limiter
	userList                []panel.UserInfo
	aliveMap                map[int]int
	conf                    *conf.NodeConfig
	info                    *panel.NodeInfo
	nodeInfoMonitorPeriodic *task.Task
	userReportPeriodic      *task.Task
	renewCertPeriodic       *task.Task
}

// NewController return a Node controller with default parameters.
func NewController(api *panel.Client, conf *conf.NodeConfig, info *panel.NodeInfo) *Controller {
	controller := &Controller{
		apiClient: api,
		info:      info,
		conf:      conf,
	}
	return controller
}

// Start implement the Start() function of the service interface
func (c *Controller) Start(x *core.V2Core) error {
	// Init Core
	c.server = x
	var err error
	// First fetch Node Info
	node := c.info
	if node == nil {
		c.info, err = c.apiClient.GetNodeInfo(context.Background())
		if err != nil {
			return fmt.Errorf("get node info error: %s", err)
		}
		node = c.info
	}
	// Update user
	c.userList, err = c.apiClient.GetUserList(context.Background())
	if err != nil {
		return fmt.Errorf("get user list error: %s", err)
	}
	if len(c.userList) == 0 {
		return errors.New("add users error: not have any user")
	}
	c.aliveMap, err = c.apiClient.GetUserAlive(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get user alive list: %s", err)
	}
	c.tag = node.Tag

	// add limiter
	l := limiter.AddLimiter(c.info.Type, c.tag, c.userList, c.aliveMap)
	c.limiter = l
	if node.Security == panel.Tls {
		err = c.requestCert()
		if err != nil {
			return fmt.Errorf("request cert error: %s", err)
		}
	}
	// Add new tag
	err = c.server.AddNode(c.tag, node)
	if err != nil {
		return fmt.Errorf("add new node error: %s", err)
	}
	added, err := c.server.AddUsers(&core.AddUsersParams{
		Tag:      c.tag,
		Users:    c.userList,
		NodeInfo: node,
	})
	if err != nil {
		return fmt.Errorf("add users error: %s", err)
	}
	log.WithField("tag", c.tag).Infof("Added %d new users", added)
	c.info = node
	c.startTasks(node)
	return nil
}

// Close implement the Close() function of the service interface
func (c *Controller) Close() error {
	limiter.DeleteLimiter(c.tag)
	c.stopTasks()
	err := c.server.DelNode(c.tag)
	if err != nil {
		return fmt.Errorf("del node error: %s", err)
	}
	return nil
}

func (c *Controller) stopTasks() {
	if c.nodeInfoMonitorPeriodic != nil {
		c.nodeInfoMonitorPeriodic.Close()
		c.nodeInfoMonitorPeriodic = nil
	}
	if c.userReportPeriodic != nil {
		c.userReportPeriodic.Close()
		c.userReportPeriodic = nil
	}
	if c.renewCertPeriodic != nil {
		c.renewCertPeriodic.Close()
		c.renewCertPeriodic = nil
	}
}

// hotReloadNode applies panel node config changes to this node only.
// Other nodes keep their sing-box instance and active connections.
// On failure it restores the previous node so siblings are not taken down.
func (c *Controller) hotReloadNode(newN *panel.NodeInfo) error {
	if newN == nil {
		return fmt.Errorf("nil node info")
	}
	oldTag := c.tag
	oldInfo := c.info
	oldUsers := c.userList
	oldAlive := c.aliveMap

	// Pause periodic tasks for this controller; restart after swap.
	c.stopTasks()

	restore := func(cause error) error {
		c.tag = oldTag
		c.info = oldInfo
		c.userList = oldUsers
		c.aliveMap = oldAlive
		if newN.Tag != "" && newN.Tag != oldTag {
			_ = c.server.DelNode(newN.Tag)
		}
		limiter.DeleteLimiter(newN.Tag)
		limiter.DeleteLimiter(oldTag)
		if oldInfo != nil {
			c.limiter = limiter.AddLimiter(oldInfo.Type, oldTag, oldUsers, oldAlive)
			if err := c.server.AddNode(oldTag, oldInfo); err != nil {
				return fmt.Errorf("%v (also restore old node failed: %w)", cause, err)
			}
			if _, err := c.server.AddUsers(&core.AddUsersParams{
				Tag:      oldTag,
				Users:    oldUsers,
				NodeInfo: oldInfo,
			}); err != nil {
				return fmt.Errorf("%v (also restore old users failed: %w)", cause, err)
			}
		}
		return cause
	}

	c.info = newN
	if newN.Security == panel.Tls {
		if err := c.requestCert(); err != nil {
			c.info = oldInfo
			return restore(fmt.Errorf("request cert error: %s", err))
		}
	}

	limiter.DeleteLimiter(oldTag)

	if oldTag != newN.Tag {
		if err := c.server.DelNode(oldTag); err != nil {
			return restore(fmt.Errorf("del old node %s: %s", oldTag, err))
		}
	}

	c.tag = newN.Tag
	c.limiter = limiter.AddLimiter(c.info.Type, c.tag, c.userList, c.aliveMap)

	if err := c.server.AddNode(c.tag, newN); err != nil {
		return restore(fmt.Errorf("add node error: %s", err))
	}
	added, err := c.server.AddUsers(&core.AddUsersParams{
		Tag:      c.tag,
		Users:    c.userList,
		NodeInfo: newN,
	})
	if err != nil {
		_ = c.server.DelNode(c.tag)
		return restore(fmt.Errorf("add users error: %s", err))
	}
	log.WithField("tag", c.tag).Infof("Hot reloaded node (%d users), replaced %s", added, oldTag)
	// Tasks are restarted by nodeInfoMonitor after this call returns,
	// so we do not start a second monitor while the current one is still running.
	return nil
}
