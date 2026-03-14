package mtcws

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsCoreCtxClient struct {
	*WsCoreCtx
	Engine *nbhttp.Engine
}

func (wsconn *WsCoreCtxClient) Init() {
	wsconn.WsCoreCtx.Anonymous = true // <- must be true in client mode
	wsconn.WsCoreCtx.Init()

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
	var authorization string

	dialer := websocket.Dialer{
		Engine:      wsconn.Engine,
		Upgrader:    wsconn.WsUpgrader,
		DialTimeout: wsconn.ConnectTimeout,
	}

	if ext != nil {
		if ext.Proxy != nil {
			dialer.Proxy = http.ProxyURL(ext.Proxy)
		}
		if ext.Authorization != "" {
			authorization = ext.Authorization
		}
	}

	if authorization == "" {
		authorization = uuid.NewString()
	}

	c, _, err := dialer.Dial(wsurl, headers)
	if err != nil {
		if c != nil {
			return nil, c.Close()
		}
		return nil, err
	}

	return wsconn.InitConnCtx(c, authorization, wsurl, protocol, nil)
}
