package mtcrtc

import (
	"context"
)

type RTCCoreCtxServer struct {
	RTCCoreCtx
}

func (corectx *RTCCoreCtxServer) Init() {
	corectx.RTCCoreCtx.Server = true
	corectx.RTCCoreCtx.Init()
}

func (corectx *RTCCoreCtxServer) InitServerConn(ctx context.Context, nodeID, connType, protocol string) (*RTCConnContext, *RTCSignal) {
	responseSignal := &RTCSignal{
		ID:   connType + ":" + nodeID,
		Type: "ack",
	}

	conn := corectx.InitWebRTC(ctx, nodeID, connType, protocol)
	if conn != nil {
		r, err := conn.CreateOffer()
		if err != nil {
			return conn, responseSignal
		}
		responseSignal.Type = r.Type
		responseSignal.Sdp = r.Sdp
	}

	return conn, responseSignal
}
