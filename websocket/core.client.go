package mtcws

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lesismal/nbio/nbhttp"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsCoreCtxClient struct {
	WsCoreCtx
	Engine *nbhttp.Engine
}

func (wsconn *WsCoreCtxClient) Init() {
	wsconn.WsCoreCtx.Init()

	wsconn.Engine = nbhttp.NewEngine(nbhttp.Config{})
	wsconn.Engine.Start()
}

func (wsconn *WsCoreCtxClient) Stop() error {
	wsconn.WsCoreCtx.Stop()
	wsconn.Engine.Stop()
	return nil
}

func (wsconn *WsCoreCtxClient) WebsocketClient(ctx context.Context, _url string, headers http.Header) (*WsConnContext, error) {
	protocol := AutoResponseProtocol(headers.Get("Sec-WebSocket-Protocol"))
	authorization := strings.ReplaceAll(headers.Get("Authorization"), "Bearer ", "")

	if authorization == "" {
		authorization = uuid.NewString()
	}

	dialer := websocket.Dialer{
		Engine:      wsconn.Engine,
		Upgrader:    wsconn.WsUpgrader,
		DialTimeout: time.Second * 10,
	}

	store, ok := ctx.Value("mtc-store").(map[string]string)
	if ok {
		if proxy := store["proxy"]; proxy != "" {
			if proxyURL, err := url.Parse(proxy); err == nil {
				dialer.Proxy = http.ProxyURL(proxyURL)
			}
		}
	}

	c, _, err := dialer.Dial(_url, headers)
	if err != nil {
		return nil, err
	}

	//defer c.Close()
	log.Println(c.RemoteAddr().String(), "connected")

	return wsconn.InitConn(ctx, c, authorization, "client", protocol), nil
}
