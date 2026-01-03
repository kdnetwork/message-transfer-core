package mtcws

import (
	"context"
	"log/slog"
	"sync"

	"github.com/jellydator/ttlcache/v3"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsConnContext struct {
	Conn     *websocket.Conn
	Addr     string
	ID       string
	ConnType string
	Protocol string
	Store    map[string]string

	Ext *WsCoreCtx

	Ctx         context.Context
	Cancel      context.CancelFunc
	CloseAction sync.Once
}

func (corectx *WsCoreCtx) InitConn(_ctx context.Context, c *websocket.Conn, nodeID, connType string, protocol string) *WsConnContext {
	ctx, cancel := context.WithCancel(_ctx)

	var store map[string]string
	var ok bool

	if store, ok = ctx.Value("mtc-store").(map[string]string); !ok {
		store = make(map[string]string)
	}

	connCtx := &WsConnContext{
		Conn:     c,
		Addr:     c.RemoteAddr().String(),
		ID:       nodeID,
		ConnType: connType,
		Ext:      corectx,
		Ctx:      ctx,
		Cancel:   cancel,
		Protocol: AutoResponseProtocol(protocol),
		Store:    store,
	}
	go connCtx.Close()

	c.SetSession(connCtx)

	connKey := connCtx.ConnType + ":" + connCtx.ID

	// disconnect
	corectx.WebsocketConnPool.Delete(connKey)
	corectx.WebsocketConnPool.Set(connKey, connCtx, ttlcache.DefaultTTL)

	//go connCtx.Conn.HandleRead(4096)

	return connCtx
}

func (wsconn *WsConnContext) Close() {
	<-wsconn.Ctx.Done()
	wsconn.CloseAction.Do(func() {
		connID := wsconn.ConnType + ":" + wsconn.ID

		defer slog.Info("mtcws", "id", connID, "ip", wsconn.Conn.RemoteAddr().String(), "status", "closed")

		if wsconn.Ext.OnDisConnected != nil {
			wsconn.Ext.OnDisConnected(wsconn)
		}

		wsconn.Conn.Close()
	})
}
