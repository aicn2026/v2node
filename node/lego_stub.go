//go:build !acme

package node

import (
	"fmt"

	panel "github.com/wyx2685/v2node/api/v2board"
)

// Lego is a stub when ACME is not built in (default).
type Lego struct {
	config *panel.CertInfo
}

func NewLego(config *panel.CertInfo) (*Lego, error) {
	return nil, fmt.Errorf("ACME/lego disabled in this build; use cert_mode none/file/self/remote, or rebuild with -tags acme")
}

func (l *Lego) CreateCert() error {
	return fmt.Errorf("ACME/lego disabled in this build; rebuild with -tags acme")
}

func (l *Lego) RenewCert() error {
	return fmt.Errorf("ACME/lego disabled in this build; rebuild with -tags acme")
}
