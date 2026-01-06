package mtcrtc

import (
	"context"
	"log/slog"
	"time"

	"github.com/jellydator/ttlcache/v3"
	"github.com/pion/webrtc/v4"
)

type RTCCoreCtx struct {
	// variables
	Server      bool // server only
	Ctx         context.Context
	Cancel      context.CancelFunc
	TTL         time.Duration
	ConnPoolTTL time.Duration
	ConnSize    uint64

	ConnTimeout time.Duration

	// pool
	WebRTCConnPool *ttlcache.Cache[string, *RTCConnContext]

	// event
	OnConnected    func(*RTCConnContext) error
	OnDisConnected func(*RTCConnContext) error
	OnMessage      func(*RTCConnContext, []byte) ([]byte, error)

	// ice
	ICEServerURLs []string
}

type RTCSignal struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Sdp          string                   `json:"sdp,omitempty"`
	ICECandidate *webrtc.ICECandidateInit `json:"ice_candidate,omitempty"`
}

func (corectx *RTCCoreCtx) Init() {
	corectx.Ctx, corectx.Cancel = context.WithCancel(context.Background())

	// to use empty ICEServerURLs, set []string{}
	if corectx.ICEServerURLs == nil {
		corectx.ICEServerURLs = []string{
			"stun:stun.cloudflare.com:3478",
			"stun:stun.l.google.com:19302",
		}
	}

	if corectx.TTL == 0 {
		corectx.TTL = time.Hour * 24
	}

	corectx.WebRTCConnPool = ttlcache.New(
		ttlcache.WithCapacity[string, *RTCConnContext](corectx.ConnSize), // ?
		ttlcache.WithTTL[string, *RTCConnContext](corectx.TTL),
		ttlcache.WithDisableTouchOnHit[string, *RTCConnContext](),
	)

	// conn pool
	corectx.WebRTCConnPool.OnEviction(func(ctx context.Context, reason ttlcache.EvictionReason, i *ttlcache.Item[string, *RTCConnContext]) {
		connCtx := i.Value()
		defer connCtx.Cancel()
		if connCtx.Peer != nil && connCtx.MainChannel != nil {
			if i.IsExpired() {
				connCtx.Store["disconnect_reason"] = "expired"
			} else {
				connCtx.Store["disconnect_reason"] = "kick"
			}
		}
	})

	go corectx.WebRTCConnPool.Start()
}

func (corectx *RTCCoreCtx) Stop() error {
	corectx.Cancel()
	corectx.WebRTCConnPool.DeleteAll()
	return nil
}

func (corectx *RTCCoreCtx) InitEvents(connCtx *RTCConnContext) error {
	connKey := connCtx.ConnType + ":" + connCtx.ID

	connCtx.Peer.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		if candidate != nil {
			candidateJSON := candidate.ToJSON()
			connCtx.mu.Lock()
			defer connCtx.mu.Unlock()

			connCtx.SignalPool = append(connCtx.SignalPool, &RTCSignal{
				ID:   connKey,
				Type: "ice_candidate",
				ICECandidate: &webrtc.ICECandidateInit{
					Candidate:        candidateJSON.Candidate,
					SDPMid:           candidateJSON.SDPMid,
					SDPMLineIndex:    candidateJSON.SDPMLineIndex,
					UsernameFragment: candidateJSON.UsernameFragment,
				},
			})
		}
	})

	connCtx.Peer.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		slog.Debug("mtcrtc", "state", state, "id", connCtx.ID, "type", connCtx.ConnType)
		switch state {
		case webrtc.PeerConnectionStateConnected:
			if corectx.OnConnected != nil {
				corectx.OnConnected(connCtx)
			}
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
			corectx.WebRTCConnPool.Delete(connKey)
		}
	})

	connCtx.Peer.OnDataChannel(func(dc *webrtc.DataChannel) {
		label := dc.Label()
		slog.Debug("mtcrtc", "channel", label, "status", "created")

		if !corectx.Server {
			defer func() {
				connCtx.mu.Lock()
				defer connCtx.mu.Unlock()
				connCtx.ChannelMap[label] = dc
				if label == "main" {
					connCtx.MainChannel = dc
				}
			}()
		}

		connCtx.InitDataChannel(dc)
	})

	return nil
}

func (corectx *RTCCoreCtx) InitWebRTC(_ctx context.Context, nodeID, connType, protocol string) *RTCConnContext {
	ctx, cancel := context.WithCancel(_ctx)

	var store map[string]string
	var ok bool

	if store, ok = ctx.Value("mtc-store").(map[string]string); !ok {
		store = make(map[string]string)
	}

	connCtx := &RTCConnContext{
		ID:       nodeID,
		ConnType: connType,
		Ext:      corectx,
		Protocol: protocol,

		// LastSignal: make(chan *RTCSignal, 100),

		Ctx:    ctx,
		Cancel: cancel,
		Store:  store,

		ChannelMap: make(map[string]*webrtc.DataChannel),
	}
	go connCtx.Close()

	connKey := connCtx.ConnType + ":" + connCtx.ID

	// disconnect
	corectx.WebRTCConnPool.Delete(connKey)
	corectx.WebRTCConnPool.Set(connKey, connCtx, ttlcache.DefaultTTL)

	if corectx.ConnTimeout > 0 {
		go func() {
			select {
			case <-time.After(corectx.ConnTimeout):
				if connCtx.MainChannel == nil || connCtx.MainChannel.ReadyState() != webrtc.DataChannelStateOpen {
					connCtx.Store["disconnect_reason"] = "signaling_timeout"
					corectx.WebRTCConnPool.Delete(connKey)
				}
			case <-ctx.Done():
			}
		}()
	}

	// TODO errors
	err := connCtx.CreatePeerConnection()
	if err != nil {
		slog.Error("mtcrtc", "status", "webrtc-cerate-peer-conn", "error", err)
		corectx.WebRTCConnPool.Delete(connKey)
		return nil
	}

	if err = corectx.InitEvents(connCtx); err != nil {
		slog.Error("mtcrtc", "status", "webrtc-init-events", "error", err)
		corectx.WebRTCConnPool.Delete(connKey)
		return nil
	}

	if corectx.Server {
		if err = connCtx.CreatePeerChannel("main", false); err != nil {
			slog.Error("mtcrtc", "status", "webrtc-server-init-mai-channel", "error", err)
			corectx.WebRTCConnPool.Delete(connKey)
			return nil
		}
	}

	return connCtx
}
