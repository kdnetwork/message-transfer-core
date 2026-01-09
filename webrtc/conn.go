package mtcrtc

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"sync"

	mtcws "github.com/kdnetwork/message-transfer-core/websocket"
	"github.com/pion/webrtc/v4"
)

type RTCConnContext struct {
	Peer        *webrtc.PeerConnection
	MainChannel *webrtc.DataChannel
	ChannelMap  map[string]*webrtc.DataChannel
	ID          string
	ConnType    string
	Store       map[string]string
	SignalPool  []*RTCSignal
	Protocol    string

	Ext *RTCCoreCtx

	Ctx         context.Context
	Cancel      context.CancelFunc
	CloseAction sync.Once

	mu sync.Mutex
}

func (rtcconn *RTCConnContext) CreatePeerChannel(channelName string, ordered bool) error {
	options := &webrtc.DataChannelInit{
		Ordered: &ordered,
	}
	dataChannel, err := rtcconn.Peer.CreateDataChannel(channelName, options)
	if err != nil {
		return err
	}
	rtcconn.InitDataChannel(dataChannel)

	if channelName == "main" {
		rtcconn.MainChannel = dataChannel
	}

	rtcconn.mu.Lock()
	defer rtcconn.mu.Unlock()
	rtcconn.ChannelMap[channelName] = dataChannel

	return nil
}

func (rtcconn *RTCConnContext) InitDataChannel(dc *webrtc.DataChannel) {
	dc.OnError(func(err error) {
		slog.Error("mtcrtc", "error", err)
	})

	dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		if rtcconn.Ext.OnMessage == nil {
			return
		}

		if slices.Contains([]string{mtcws.WSPingMessageNum, mtcws.WSPingMessageStr, ""}, string(msg.Data)) {
			// yes... return void
			return
		}

		response, err := rtcconn.Ext.OnMessage(rtcconn, msg.Data)

		if err != nil {
			slog.Error("mtcrtc", "error", err)
		}
		if len(response) > 0 {
			// slog.Debug("mtcrtc", "res", response)
			if err = dc.Send(response); err != nil {
				slog.Error("mtcrtc", "error", err, "res", response)
			}
		}
	})
}

func (rtcconn *RTCConnContext) CreatePeerConnection() error {
	iceServerURLs := rtcconn.Ext.ICEServerURLs

	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: iceServerURLs,
			},
		},
	}
	var err error

	s := webrtc.SettingEngine{
		LoggerFactory: slogLoggerFactory{},
	}
	api := webrtc.NewAPI(webrtc.WithSettingEngine(s))

	rtcconn.Peer, err = api.NewPeerConnection(config)

	return err
}

func (rtcconn *RTCConnContext) CreateOffer() (*RTCSignal, error) {
	offer, err := rtcconn.Peer.CreateOffer(nil)
	if err != nil {
		return nil, err
	}

	err = rtcconn.Peer.SetLocalDescription(offer)
	if err != nil {
		return nil, err
	}

	signal := &RTCSignal{
		Type: "offer",
		Sdp:  offer.SDP,
	}

	// rtcconn.LastSignal <- signal
	return signal, nil
}

func (rtcconn *RTCConnContext) CreateAnswer() (*RTCSignal, error) {
	answer, err := rtcconn.Peer.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	err = rtcconn.Peer.SetLocalDescription(answer)
	if err != nil {
		return nil, err
	}

	signal := &RTCSignal{
		Type: "answer",
		Sdp:  answer.SDP,
	}

	// rtcconn.LastSignal <- signal
	return signal, nil
}

func (rtcconn *RTCConnContext) SetRemoteDescription(signal *RTCSignal) error {
	if signal == nil {
		return errors.New("webrtc-rpc: nil signal")
	}

	sdpType := webrtc.SDPTypeOffer
	if signal.Type == "answer" {
		sdpType = webrtc.SDPTypeAnswer
	}

	return rtcconn.Peer.SetRemoteDescription(webrtc.SessionDescription{
		SDP:  signal.Sdp,
		Type: sdpType,
	})
}

func (rtcconn *RTCConnContext) AddICECandidate(signal *RTCSignal) error {
	if signal == nil {
		return errors.New("webrtc-rpc: nil signal")
	}
	if signal.Type != "ice_candidate" {
		return errors.New("webrtc-rpc: invalid signal type")
	}

	return rtcconn.Peer.AddICECandidate(*signal.ICECandidate)
}

// don't call func Close() directly, use `rtcconn.Ext.WebRTCConnPool.Delete(rtcconn.ConnType + ":" + rtcconn.ID)`
func (rtcconn *RTCConnContext) Close() error {
	<-rtcconn.Ctx.Done()
	rtcconn.CloseAction.Do(func() {
		if rtcconn.Ext.OnDisConnected != nil {
			if err := rtcconn.Ext.OnDisConnected(rtcconn); err != nil {
				slog.Error("mtcrtc", "error", err)
			}
		}

		rtcconn.mu.Lock()
		channels := make([]*webrtc.DataChannel, 0, len(rtcconn.ChannelMap))
		for _, ch := range rtcconn.ChannelMap {
			channels = append(channels, ch)
		}
		rtcconn.mu.Unlock()

		for _, channel := range channels {
			if err := channel.Close(); err != nil {
				slog.Error("mtcrtc", "error", err)
			}
		}

		if rtcconn.Peer != nil {
			if err := rtcconn.Peer.Close(); err != nil {
				slog.Error("mtcrtc", "error", err)
			}
		}
		// close(rtcconn.LastSignal)
	})
	return nil
}

func (rtcconn *RTCConnContext) SwapSignal(signal *RTCSignal) *RTCSignal {
	responseSignal := &RTCSignal{
		ID:   rtcconn.ConnType + ":" + rtcconn.ID,
		Type: "ack",
	}

	// `init` use corectx.InitServerConn()
	if signal == nil || !slices.Contains([]string{"answer", "offer", "ice_candidate"}, signal.Type) {
		return responseSignal // invalid signal type
	}

	switch signal.Type {
	case "offer":
		rtcconn.SetRemoteDescription(&RTCSignal{
			Type: signal.Type,
			Sdp:  signal.Sdp,
		})
		r, err := rtcconn.CreateAnswer()
		if err != nil {
			return responseSignal
		}
		responseSignal.Type = r.Type
		responseSignal.Sdp = r.Sdp
	case "answer":
		rtcconn.SetRemoteDescription(&RTCSignal{
			Type: signal.Type,
			Sdp:  signal.Sdp,
		})
	case "ice_candidate":
		if signal.ICECandidate == nil {
			return responseSignal
		}
		err := rtcconn.AddICECandidate(&RTCSignal{
			Type: signal.Type,
			ICECandidate: &webrtc.ICECandidateInit{
				Candidate:        signal.ICECandidate.Candidate,
				SDPMid:           signal.ICECandidate.SDPMid,
				SDPMLineIndex:    signal.ICECandidate.SDPMLineIndex,
				UsernameFragment: signal.ICECandidate.UsernameFragment,
			},
		})
		if err != nil {
			return responseSignal
		}
	}

	if responseSignal.Type != "ack" {
		rtcconn.mu.Lock()
		defer rtcconn.mu.Unlock()

		rtcconn.SignalPool = append(rtcconn.SignalPool, responseSignal)
	}

	return responseSignal
}

func (rtcconn *RTCConnContext) ExportSignalPool() []*RTCSignal {
	rtcconn.mu.Lock()
	defer rtcconn.mu.Unlock()

	if len(rtcconn.SignalPool) == 0 {
		return []*RTCSignal{}
	}

	resPool := rtcconn.SignalPool
	rtcconn.SignalPool = []*RTCSignal{}

	return resPool
}
