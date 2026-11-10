package mtcws

import (
	"context"
	"net/http"

	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsCoreCtxClient struct {
	*WsCoreCtx
	Engine *nbhttp.Engine
}

func (wsconn *WsCoreCtxClient) Init(ctx context.Context) {
	wsconn.WsCoreCtx.Anonymous = true // <- must be true in client mode
	wsconn.WsCoreCtx.Init(ctx)

	wsconn.Engine = nbhttp.NewEngine(nbhttp.Config{})
	wsconn.Engine.Start()
}

func (wsconn *WsCoreCtxClient) Stop() error {
	wsconn.WsCoreCtx.Stop()
	wsconn.Engine.Stop()
	return nil
}

func (wsconn *WsCoreCtxClient) WebsocketClient(wsurl string, headers http.Header, ext *WsConnConfigExt) (*WsConnContext, error) {
	protocol := headers.Get("Sec-WebSocket-Protocol")

	dialer := websocket.Dialer{
		Engine:      wsconn.Engine,
		Upgrader:    wsconn.WsUpgrader,
		DialTimeout: wsconn.ConnectTimeout,
	}

	nodeID := ""

	var store = make(map[string]string)

	if ext != nil {
		if ext.Proxy != nil {
			dialer.Proxy = http.ProxyURL(ext.Proxy)
		}
		if ext.NodeID != "" {
			nodeID = ext.NodeID
		}
		if ext.Store != nil {
			store = ext.Store
		}
	}

	c, _, err := dialer.Dial(wsurl, headers)
	if err != nil {
		if c != nil {
			return nil, c.Close()
		}
		return nil, err
	}

	return wsconn.InitConnCtx(c, nodeID, wsurl, protocol, store)
}
