package gateway

import (
	"github.com/diamondburned/arikawa/v3/api"
	discordgateway "github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"libdb.so/dissent/internal/http"
)

const gatewayURL = "wss://gateway.discord.gg"

func New(id discordgateway.Identifier) *discordgateway.Gateway {
	codec := ws.NewCodec(discordgateway.OpUnmarshalers)
	codec.Headers.Set("Origin", api.BaseEndpoint)
	codec.Headers.Set("User-Agent", http.BrowserUserAgent())

	conn := ws.NewConnWithDialer(codec, NewDialer())
	socket := ws.NewCustomWebsocket(conn, discordgateway.AddGatewayParams(gatewayURL))
	return discordgateway.FromWebsocketGateway(ws.NewGateway(socket, &discordgateway.DefaultGatewayOpts), discordgateway.State{Identifier: id})
}
