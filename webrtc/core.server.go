package mtcrtc

type RTCCoreCtxServer struct {
	RTCCoreCtx
}

func (corectx *RTCCoreCtxServer) Init() {
	corectx.RTCCoreCtx.Server = true
	corectx.RTCCoreCtx.Init()
}

func (corectx *RTCCoreCtxServer) InitServerConn(nodeID, connType, protocol string, store map[string]string) (*RTCConnContext, *RTCSignal) {
	responseSignal := &RTCSignal{
		ID:   connType + ":" + nodeID,
		Type: "ack",
	}

	conn := corectx.InitWebRTC(nodeID, connType, protocol, store)
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
