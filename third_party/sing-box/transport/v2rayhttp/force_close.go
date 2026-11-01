package v2rayhttp

import (
	"net/http"
	"reflect"

	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/net/http2"
)

// ResetTransport closes idle connections. Avoids go:linkname into http2.Transport.connPool
// which breaks against newer golang.org/x/net used by lego/grpc.
func ResetTransport(rawTransport http.RoundTripper) http.RoundTripper {
	switch transport := rawTransport.(type) {
	case *http.Transport:
		transport.CloseIdleConnections()
		return transport.Clone()
	case *http2.Transport:
		transport.CloseIdleConnections()
		return transport
	default:
		panic(E.New("unknown transport type: ", reflect.TypeOf(transport)))
	}
}
