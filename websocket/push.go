package mtcws

import (
	"errors"

	"github.com/jellydator/ttlcache/v3"
	"github.com/lesismal/nbio/nbhttp/websocket"
)

// SendWebsocketMessage when `wsconn.Ctx` was done, `SendWebsocketMessage` will not work, use hack code `wsconn.Conn.WriteMessage(...)` to post last message before disconnect, in func OnDisconnected() ...
func (wsconn *WsConnContext) SendWebsocketMessage(data []byte) error {
	if wsconn.Conn == nil {
		return errors.New("no conn")
	}
	if err := wsconn.Ctx.Err(); err != nil {
		return err
	}
	if wsconn.Protocol == "json" {
		return wsconn.Conn.WriteMessage(websocket.TextMessage, data)
	}

	return wsconn.Conn.WriteMessage(websocket.BinaryMessage, data)
}

func (corectx *WsCoreCtx) BroadcastToWebSocket(data []byte, connTypeFilter string) map[string]error {
	errs := make(map[string]error)
	corectx.WebsocketConnPool.Range(func(item *ttlcache.Item[string, *WsConnContext]) bool {
		conn := item.Value()
		if conn != nil {
			if connTypeFilter != "" && conn.ConnType != connTypeFilter {
				return true
			}
			errs[item.Key()] = conn.SendWebsocketMessage(data)
		}
		return true
	})

	return errs
}
