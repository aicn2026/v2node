package core

import (
	"fmt"

	panel "github.com/wyx2685/v2node/api/v2board"
)

func (v *V2Core) AddNode(tag string, info *panel.NodeInfo) error {
	if err := v.engine.AddNode(tag, info); err != nil {
		return fmt.Errorf("add node error: %s", err)
	}
	return nil
}

func (v *V2Core) DelNode(tag string) error {
	if err := v.engine.DelNode(tag); err != nil {
		return fmt.Errorf("del node error: %s", err)
	}
	return nil
}
