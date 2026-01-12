package mtcws

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
)

func (wsconn *WsCoreCtx) WebsocketServer(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	store, ok := ctx.Value("mtc-store").(map[string]string)
	var nodeID, connType string
	if !ok && !wsconn.Anonymous {
		return errors.New("invalid store")
	} else if ok && !wsconn.Anonymous {
		nodeID = store["node_id"]
		connType = store["conn_type"]
	} else {
		nodeID = uuid.NewString()
		connType = "anonymous"
	}

	if nodeID == "" || connType == "" {
		return errors.New("invalid node-id or conn-type")
	}

	if err := wsconn.Ctx.Err(); err != nil {
		return err
	}

	c, err := wsconn.WsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// slog.Error("upgrade:", err)
		if c != nil {
			return c.Close()
		}
		return err
	}

	_, err = wsconn.InitConnCtx(ctx, c, nodeID, connType, c.Subprotocol(), store)

	return err
}
