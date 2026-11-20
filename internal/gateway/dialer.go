//go:build !no_spoof_tls_fingerprint

package gateway

import (
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/gorilla/websocket"
	"libdb.so/dissent/internal/tls"
)

func NewDialer() websocket.Dialer {
	client := tls.NewClient(tls_client.WithForceHttp1())
	dialer := *websocket.DefaultDialer
	dialer.Proxy = nil
	dialer.EnableCompression = true
	dialer.NetDialTLSContext = client.GetTLSDialer()
	return dialer
}
