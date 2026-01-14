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
}

func (corectx *WsCoreCtx) InitConnCtx(_ctx context.Context, c *websocket.Conn, nodeID, connType string, protocol string, store map[string]string) (*WsConnContext, error) {
	ctx, cancel := context.WithTimeout(_ctx, corectx.TTL)
	connKey := connType + ":" + nodeID
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

	savedUUID, err, _ := corectx.ConnSf.Do(connKey, func() (any, error) {
		if item := corectx.WebsocketConnPool.Get(connKey); item != nil {
			if existsConn := item.Value(); existsConn != nil && existsConn.Conn != nil {
				existsConn.Store["disconnect_reason"] = "kick"
				existsConn.Cancel()
			}
		}
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
		connCtx.Cancel()
		return nil, errors.New("duplicate connection")
	}

	return connCtx, nil
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
