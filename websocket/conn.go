package mtcws

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

type WsConnContext struct {
	ConnUUID    string
	Conn        *websocket.Conn
	ID          string
	ConnType    string
	Protocol    string
	Store       map[string]string
	ConnectedAt time.Time

	Ext *WsCoreCtx

	Ctx         context.Context
	Cancel      context.CancelFunc
	CloseAction sync.Once

	mu sync.RWMutex
}

func (connctx *WsConnContext) SetStore(key, value string) {
	connctx.mu.Lock()
	defer connctx.mu.Unlock()
	if connctx.Store == nil {
		connctx.Store = make(map[string]string)
	}
	connctx.Store[key] = value
}

func (connctx *WsConnContext) GetStore(key string) (string, bool) {
	connctx.mu.RLock()
	defer connctx.mu.RUnlock()
	val, ok := connctx.Store[key]
	return val, ok
}

func (connctx *WsConnContext) ConnKey() string {
	return connctx.ConnType + ":" + connctx.ID
}

func (corectx *WsCoreCtx) InitConnCtx(_ctx context.Context, c *websocket.Conn, nodeID, connType string, protocol string, store map[string]string) (*WsConnContext, error) {
	ctx, cancel := context.WithTimeout(_ctx, corectx.TTL)
	connCtx := &WsConnContext{
		ConnUUID:    uuid.NewString(),
		Conn:        c,
		ID:          nodeID,
		ConnType:    connType,
		Ext:         corectx,
		Ctx:         ctx,
		Cancel:      cancel,
		Protocol:    AutoResponseProtocol(protocol),
		Store:       store,
		ConnectedAt: time.Now(),
	}

	c.SetSession(connCtx)

	connKey := connCtx.ConnKey()

	savedUUID, err, _ := corectx.ConnSf.Do(connKey, func() (any, error) {
		corectx.WebsocketConnPool.Delete(connKey)

		corectx.WebsocketConnPool.Set(connKey, connCtx, ttlcache.DefaultTTL)
		go connCtx.Close()
		slog.Debug("mtcws", c.RemoteAddr().String(), "connected")

		if corectx.OnConnected != nil {
			return connCtx.ConnUUID, corectx.OnConnected(connCtx)
		}

		return connCtx.ConnUUID, nil

	})

	if err != nil {
		connCtx.Cancel()
		return nil, err
	} else if savedUUID != connCtx.ConnUUID {
		store["cancel_ctx_by"] = "duplicate_connection"
		connCtx.Cancel()
		return nil, errors.New("duplicate connection")
	}

	return connCtx, nil
}

func (wsconn *WsConnContext) Close() {
	<-wsconn.Ctx.Done()
	wsconn.CloseAction.Do(func() {
		connKey := wsconn.ConnKey()

		defer slog.Info("mtcws", "id", connKey, "ip", wsconn.Conn.RemoteAddr().String(), "status", "closed")

		if wsconn.Ext.OnDisConnected != nil {
			wsconn.Ext.OnDisConnected(wsconn)
		}

		if wsconn.Store != nil {
			// when `cancel_ctx_by` exists, do not delete from pool
			if by, exists := wsconn.Store["cancel_ctx_by"]; !exists || by == "" {
				wsconn.Ext.WebsocketConnPool.Delete(connKey)
			}
		}

		wsconn.Conn.Close()
	})
}
