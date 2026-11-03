package node

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	panel "github.com/wyx2685/v2node/api/v2board"
	"github.com/wyx2685/v2node/conf"
	"github.com/wyx2685/v2node/core"
)

type Node struct {
	controllers []*Controller
	NodeInfos   []*panel.NodeInfo
}

func nodeKey(c conf.NodeConfig) string {
	return fmt.Sprintf("%s|%d", c.APIHost, c.NodeID)
}

func nodeConfigEqual(a, b conf.NodeConfig) bool {
	if a.APIHost != b.APIHost || a.NodeID != b.NodeID || a.Key != b.Key || a.Timeout != b.Timeout {
		return false
	}
	ar, br := conf.DefaultNodeRetryCount, conf.DefaultNodeRetryCount
	if a.RetryCount != nil {
		ar = *a.RetryCount
	}
	if b.RetryCount != nil {
		br = *b.RetryCount
	}
	return ar == br
}

func New(nodes []conf.NodeConfig) (*Node, error) {
	n := &Node{
		controllers: make([]*Controller, 0, len(nodes)),
		NodeInfos:   make([]*panel.NodeInfo, 0, len(nodes)),
	}
	for _, node := range nodes {
		cfg := node
		p, err := panel.New(&cfg)
		if err != nil {
			return nil, err
		}
		info, err := p.GetNodeInfo(context.Background())
		if err != nil {
			return nil, err
		}
		n.controllers = append(n.controllers, NewController(p, &cfg, info))
		n.NodeInfos = append(n.NodeInfos, info)
	}
	return n, nil
}

func (n *Node) Start(nodes []conf.NodeConfig, core *core.V2Core) error {
	for i, node := range nodes {
		err := n.controllers[i].Start(core)
		if err != nil {
			return fmt.Errorf("start node controller [%s-%d] error: %s",
				node.APIHost,
				node.NodeID,
				err)
		}
	}
	return nil
}

func (n *Node) Close() error {
	var err error
	for _, c := range n.controllers {
		if err = c.Close(); err != nil {
			log.Errorf("close controller failed: %v", err)
			return err
		}
	}
	n.controllers = nil
	n.NodeInfos = nil
	return nil
}

// SyncConfig applies config.json node list changes without touching unchanged nodes.
// Added nodes are started; removed nodes are stopped; credential changes restart that node only.
func (n *Node) SyncConfig(cfgs []conf.NodeConfig, v2core *core.V2Core) error {
	if n == nil {
		return fmt.Errorf("nil node manager")
	}
	if v2core == nil {
		return fmt.Errorf("nil core")
	}

	running := make(map[string]*Controller, len(n.controllers))
	for _, c := range n.controllers {
		if c == nil || c.conf == nil {
			continue
		}
		running[nodeKey(*c.conf)] = c
	}

	desired := make(map[string]conf.NodeConfig, len(cfgs))
	order := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		k := nodeKey(cfg)
		if _, dup := desired[k]; dup {
			return fmt.Errorf("duplicate node in config: %s NodeID=%d", cfg.APIHost, cfg.NodeID)
		}
		desired[k] = cfg
		order = append(order, k)
	}

	// Remove nodes no longer present.
	kept := make([]*Controller, 0, len(order))
	keptInfos := make([]*panel.NodeInfo, 0, len(order))
	for _, c := range n.controllers {
		if c == nil || c.conf == nil {
			continue
		}
		k := nodeKey(*c.conf)
		if _, ok := desired[k]; ok {
			continue
		}
		log.WithField("node", k).Info("Config removed node, stopping")
		if err := c.Close(); err != nil {
			return fmt.Errorf("stop removed node [%s]: %w", k, err)
		}
		delete(running, k)
	}

	// Add or keep/replace in desired order.
	for _, k := range order {
		cfg := desired[k]
		cfgCopy := cfg
		old, exists := running[k]
		if exists && old.conf != nil && nodeConfigEqual(*old.conf, cfgCopy) {
			kept = append(kept, old)
			keptInfos = append(keptInfos, old.info)
			continue
		}
		if exists {
			log.WithField("node", k).Info("Config changed node credentials, restarting this node only")
			if err := old.Close(); err != nil {
				return fmt.Errorf("stop changed node [%s]: %w", k, err)
			}
			delete(running, k)
		} else {
			log.WithField("node", k).Info("Config added node, starting")
		}

		p, err := panel.New(&cfgCopy)
		if err != nil {
			return fmt.Errorf("create panel client [%s]: %w", k, err)
		}
		info, err := p.GetNodeInfo(context.Background())
		if err != nil {
			return fmt.Errorf("get node info [%s]: %w", k, err)
		}
		ctrl := NewController(p, &cfgCopy, info)
		if err := ctrl.Start(v2core); err != nil {
			return fmt.Errorf("start node controller [%s-%d]: %w", cfgCopy.APIHost, cfgCopy.NodeID, err)
		}
		kept = append(kept, ctrl)
		keptInfos = append(keptInfos, info)
	}

	n.controllers = kept
	n.NodeInfos = keptInfos
	return nil
}
