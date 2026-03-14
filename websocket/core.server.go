package mtcws

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
)

func (wsconn *WsCoreCtx) WebsocketServer(w http.ResponseWriter, r *http.Request, ext *WsConnConfigExt) error {
	// <- ext maybe nil
	if ext == nil && !wsconn.Anonymous {
		return errors.New("invalid user")
	}

	var nodeID, connType string
	if wsconn.Anonymous {
		nodeID = uuid.NewString()
		connType = "anonymous"
	} else {
		nodeID = ext.NodeID
		connType = ext.ConnType
	}

	if nodeID == "" || connType == "" {
		return errors.New("invalid node-id or conn-type")
	}

	if err := wsconn.Ctx.Err(); err != nil {
		return err
	}

	connKey := connType + ":" + nodeID

	connUUID := uuid.NewString()

	savedUUID, err, _ := wsconn.ConnSf.Do(connKey, func() (any, error) {
		c, err := wsconn.WsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			// slog.Error("upgrade:", err)
			if c != nil {
				return connUUID, c.Close()
			}
			return connUUID, err
		}

		_, err = wsconn.InitConnCtx(c, nodeID, connType, c.Subprotocol(), nil)

		return connUUID, err
	})

	if savedUUID != connUUID {
		return errors.New("duplicate connection")
	}

	return err
}
