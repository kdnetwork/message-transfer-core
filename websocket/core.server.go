package mtcws

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
)

type WsServerSfResponse struct {
	ConnUUID string
	ConnCtx  *WsConnContext
}

func (wsconn *WsCoreCtx) WebsocketServer(w http.ResponseWriter, r *http.Request, ext *WsConnConfigExt) (*WsConnContext, error) {
	// <- ext maybe nil
	var nodeID, connType string

	var store = make(map[string]string)

	if ext == nil {
		if !wsconn.Anonymous {
			return nil, errors.New("invalid user")
		}

		nodeID = uuid.NewString()
		connType = "anonymous"
	} else {
		nodeID = ext.NodeID
		connType = ext.ConnType
		if ext.Store != nil {
			store = ext.Store
		}
	}

	if nodeID == "" || connType == "" {
		return nil, errors.New("invalid node-id or conn-type")
	}

	if err := wsconn.Ctx.Err(); err != nil {
		return nil, err
	}

	connKey := connType + ":" + nodeID

	connUUID := uuid.NewString()

	res, err, _ := wsconn.ConnSf.Do(connKey, func() (any, error) {
		sfRes := &WsServerSfResponse{
			ConnUUID: connUUID,
		}
		c, err := wsconn.WsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			// slog.Error("upgrade:", err)
			if c != nil {
				return connUUID, c.Close()
			}
			return sfRes, err
		}

		sfRes.ConnCtx, err = wsconn.InitConnCtx(c, nodeID, connType, c.Subprotocol(), store)

		return sfRes, err
	})

	response := res.(*WsServerSfResponse)

	if response.ConnUUID != connUUID {
		return nil, errors.New("duplicate connection")
	}

	return response.ConnCtx, err
}
