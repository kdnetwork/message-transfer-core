package mtcrtc

import (
	"errors"
	"log/slog"

	"github.com/jellydator/ttlcache/v3"
	"github.com/pion/webrtc/v4"
)

func (rtcconn *RTCConnContext) SendRTCMessage(data []byte) error {
	if rtcconn.Peer == nil || rtcconn.MainChannel == nil || rtcconn.MainChannel.ReadyState() != webrtc.DataChannelStateOpen {
		return errors.New("no conn")
	}

	if err := rtcconn.Ctx.Err(); err != nil {
		return err
	}

	return rtcconn.MainChannel.Send(data)
}

func (corectx *RTCCoreCtx) BroadcastToRTC(data []byte, connTypeFilter string) {
	corectx.WebRTCConnPool.Range(func(item *ttlcache.Item[string, *RTCConnContext]) bool {
		conn := item.Value()
		if conn != nil {
			if connTypeFilter != "" && conn.ConnType != connTypeFilter {
				return true
			}
			slog.Debug("mtcrtc", item.Key(), data)
			conn.SendRTCMessage(data)
		}
		return true
	})
}
