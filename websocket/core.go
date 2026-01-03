package mtcws

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsCoreCtx struct {
	// variables
	Anonymous      bool // server only
	Ctx            context.Context
	Cancel         context.CancelFunc
	TTL            time.Duration
	ConnPoolTTL    time.Duration
	ConnectTimeout time.Duration
	ConnSize       uint64

	// nbio
	WsUpgrader        *websocket.Upgrader
	WebsocketConnPool *ttlcache.Cache[string, *WsConnContext]

	// event
	OnConnected    func(*WsConnContext) error
	OnDisConnected func(*WsConnContext) error
	OnMessage      func(*WsConnContext, []byte) ([]byte, error)
}

func (corectx *WsCoreCtx) Init() {
	InitNbioLogger()
	corectx.Ctx, corectx.Cancel = context.WithCancel(context.Background())

	if corectx.TTL == 0 {
		corectx.TTL = time.Hour * 24
	}

	if corectx.ConnectTimeout == 0 {
		corectx.ConnectTimeout = time.Second * 10
	}

	corectx.WebsocketConnPool = ttlcache.New(
		ttlcache.WithCapacity[string, *WsConnContext](corectx.ConnSize), // ?
		ttlcache.WithTTL[string, *WsConnContext](corectx.TTL),
		ttlcache.WithDisableTouchOnHit[string, *WsConnContext](),
	)

	// conn pool
	corectx.WebsocketConnPool.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, i *ttlcache.Item[string, *WsConnContext]) {
		if connCtx := i.Value(); connCtx.Conn != nil {
			defer connCtx.Cancel()

			if i.IsExpired() {
				connCtx.Store["disconnect_reason"] = "expired"
			} else {
				connCtx.Store["disconnect_reason"] = "kick"
			}
		}
	})

	go corectx.WebsocketConnPool.Start()
}

func (corectx *WsCoreCtx) Stop() error {
	corectx.Cancel()
	corectx.WebsocketConnPool.DeleteAll()
	return nil
}

func (corectx *WsCoreCtx) InitUpgrader() {
	corectx.WsUpgrader = websocket.NewUpgrader()
	corectx.WsUpgrader.KeepaliveTime = corectx.TTL + corectx.ConnectTimeout // have time to send the last message
	corectx.WsUpgrader.HandshakeTimeout = corectx.TTL + corectx.ConnectTimeout
	corectx.WsUpgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}
	corectx.WsUpgrader.Subprotocols = Protocols
	// corectx.WsUpgrader.BlockingModHandleRead = false
	// corectx.WsUpgrader.BlockingModAsyncWrite = true

	corectx.WsUpgrader.OnOpen(func(c *websocket.Conn) {
		if corectx.OnConnected == nil {
			return
		}
		wsConnContext, ok := c.SessionWithLock().(*WsConnContext)
		if !ok || wsConnContext == nil {
			return
		}

		corectx.OnConnected(wsConnContext)
	})
	corectx.WsUpgrader.OnMessage(func(c *websocket.Conn, messageType websocket.MessageType, message []byte) {
		if corectx.OnMessage == nil {
			return
		}
		wsConnContext, ok := c.SessionWithLock().(*WsConnContext)

		if !ok || wsConnContext == nil {
			return
		}

		if slices.Contains([]string{WSPingMessageNum, WSPingMessageStr, ""}, string(message)) {
			// yes... return void
			return
		}

		response, err := corectx.OnMessage(wsConnContext, message)

		if err != nil {
			slog.Error("mtcws", "error", err)
		}
		if len(response) > 0 {
			// slog.Debug(response)
			if wsConnContext.Protocol == "json" {
				c.WriteMessage(websocket.TextMessage, response)
			} else {
				c.WriteMessage(websocket.BinaryMessage, response)
			}
		}
	})
	corectx.WsUpgrader.OnClose(func(c *websocket.Conn, err error) {
		if err != nil {
			slog.Error("mtcws", "error", err)
		}
		wsConnContext, ok := c.SessionWithLock().(*WsConnContext)
		if !ok || wsConnContext == nil {
			return
		}
		wsConnContext.Cancel()
	})
}

func AutoResponseProtocol(protocol string) string {
	if !slices.Contains(Protocols, protocol) {
		protocol = "json"
	}
	return protocol
}
