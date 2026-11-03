package panel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-resty/resty/v2"
	"github.com/wyx2685/v2node/conf"
)

// Panel is the interface for different panel's api.

type Client struct {
	client           *resty.Client
	APIHost          string
	Token            string
	NodeId           int
	nodeEtag         string
	userEtag         string
	responseBodyHash string
	UserList         *UserListBody
	AliveMap         *AliveMap
}

func New(c *conf.NodeConfig) (*Client, error) {
	client := resty.New()
	retryCount := conf.DefaultNodeRetryCount
	if c.RetryCount != nil {
		retryCount = *c.RetryCount
	}
	client.SetRetryCount(retryCount)
	client.SetLogger(&restyPanelLogger{})
	client.SetHeader("User-Agent", fmt.Sprintf("v2node go-resty/%s (https://github.com/go-resty/resty)", resty.Version))
	if c.Timeout > 0 {
		client.SetTimeout(time.Duration(c.Timeout) * time.Second)
	} else {
		client.SetTimeout(time.Duration(conf.DefaultNodeTimeout) * time.Second)
	}
	client.OnError(func(req *resty.Request, err error) {
		var v *resty.ResponseError
		if errors.As(err, &v) {
			// Transient panel/network failures are expected; keep journal quiet.
			if isTransientPanelError(v.Err) {
				logrus.Debug(v.Err)
				return
			}
			logrus.Error(v.Err)
		}
	})
	client.SetBaseURL(c.APIHost)
	// set params
	client.SetQueryParams(map[string]string{
		"node_type": "v2node",
		"node_id":   strconv.Itoa(c.NodeID),
		"token":     c.Key,
	})
	return &Client{
		client:   client,
		Token:    c.Key,
		APIHost:  c.APIHost,
		NodeId:   c.NodeID,
		UserList: &UserListBody{},
		AliveMap: &AliveMap{},
	}, nil
}

func isTransientPanelError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "deadline exceeded") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "TLS handshake timeout") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "EOF")
}

// restyPanelLogger demotes transient panel API noise to debug.
type restyPanelLogger struct{}

func (l *restyPanelLogger) Errorf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if isTransientPanelError(errors.New(msg)) {
		logrus.Debug(msg)
		return
	}
	logrus.Error(msg)
}

func (l *restyPanelLogger) Warnf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if isTransientPanelError(errors.New(msg)) {
		logrus.Debug(msg)
		return
	}
	logrus.Warn(msg)
}

func (l *restyPanelLogger) Debugf(format string, v ...interface{}) {
	logrus.Debugf(format, v...)
}
