// websocket/websocket_test.go
package mtcws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellydator/ttlcache/v3"
)

const (
	ttlDuration        = time.Second
	connectTimeout     = 200 * time.Millisecond
	messageWaitTimeout = 50 * time.Millisecond
	clientTTL          = time.Second * 10
)

type TestChannels struct {
	ConnedChan    chan struct{}
	DisconnedChan chan struct{}
	MessageChan   chan []byte
	ErrorChan     chan error
}

func setupTestServer(t *testing.T, coreCtx *WsCoreCtx) (*WsCoreCtx, *httptest.Server, func(), func(), *TestChannels) {
	server := coreCtx

	chans := &TestChannels{
		ConnedChan:    make(chan struct{}, 10),
		DisconnedChan: make(chan struct{}, 10),
		MessageChan:   make(chan []byte, 10),
	}

	coreCtx.OnConnected = func(ctx *WsConnContext) error {
		chans.ConnedChan <- struct{}{}
		return nil
	}
	coreCtx.OnDisConnected = func(ctx *WsConnContext) error {
		chans.DisconnedChan <- struct{}{}
		return nil
	}
	coreCtx.OnMessage = func(ctx *WsConnContext, data []byte) ([]byte, error) {
		chans.MessageChan <- data
		return nil, nil
	}

	if server == nil {
		server = &WsCoreCtx{
			Anonymous: true,
			TTL:       ttlDuration,
		}
	}

	server.Init()
	server.InitUpgrader()

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var parts []string

		if server.Anonymous {
			parts = []string{"anonymous", uuid.NewString()}
		} else {
			authorization := r.Header.Get("Authorization")
			parts = strings.Split(authorization, ":")
			if len(parts) != 2 {
				http.Error(w, "Invalid Authorization header", http.StatusUnauthorized)
				return
			}
		}

		_, err := server.WebsocketServer(w, r, &WsConnConfigExt{
			Authorization: r.Header.Get("Authorization"), //<- test format: "user":<number-id>
			ConnType:      parts[0],
			NodeID:        parts[1],
		})
		if err != nil {
			t.Logf("server error: %v", err)
		}
	}))

	cleanup := func() {
		server.Stop()
		httpServer.Close()
		time.Sleep(50 * time.Millisecond)
	}

	resetChans := func() {
		for {
			select {
			case <-chans.ConnedChan:
			case <-chans.DisconnedChan:
			case <-chans.MessageChan:
			case <-chans.ErrorChan:
			default:
				return
			}
		}
	}

	return server, httpServer, cleanup, resetChans, chans
}

func setupTestClient(t *testing.T, coreCtx *WsCoreCtx) (*WsCoreCtxClient, func(), *TestChannels) {

	chans := &TestChannels{
		ConnedChan:    make(chan struct{}, 10),
		DisconnedChan: make(chan struct{}, 10),
		MessageChan:   make(chan []byte, 10),
	}

	if coreCtx == nil {
		coreCtx = &WsCoreCtx{
			TTL:            clientTTL,
			ConnectTimeout: connectTimeout,

			OnConnected: func(ctx *WsConnContext) error {
				chans.ConnedChan <- struct{}{}
				return nil
			},
			OnDisConnected: func(ctx *WsConnContext) error {
				chans.DisconnedChan <- struct{}{}
				return nil
			},
			OnMessage: func(ctx *WsConnContext, data []byte) ([]byte, error) {
				chans.MessageChan <- data
				return nil, nil
			},
		}
	}

	client := &WsCoreCtxClient{
		WsCoreCtx: coreCtx,
	}
	client.Init()
	client.InitUpgrader()

	cleanup := func() {
		client.Stop()
		time.Sleep(50 * time.Millisecond)
	}

	return client, cleanup, chans
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func connectClient(t *testing.T, client *WsCoreCtxClient, addr string, headers http.Header) *WsConnContext {
	conn, err := client.WebsocketClient(addr, headers, nil)
	if err != nil {
		t.Fatalf("connect failed: %v", err)
	}
	return conn
}

func expectSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatal(msg)
	}
}

func expectMessage(t *testing.T, ch <-chan []byte, expected []byte, timeout time.Duration) {
	select {
	case msg := <-ch:
		if string(msg) != string(expected) {
			t.Fatalf("unexpected message: %s, expected: %s", string(msg), string(expected))
		}
	case <-time.After(timeout):
		t.Fatal("message receive timeout")
	}
}

func expectNoSignal(t *testing.T, ch <-chan struct{}, timeout time.Duration, msg string) {
	select {
	case <-ch:
		t.Fatal(msg)
	case <-time.After(timeout):
	}
}

// basic tests

func TestWebsocketAnonymousServer(t *testing.T) {
	serverCtx, httpServer, serverCleanup, resetChans, chans := setupTestServer(t, &WsCoreCtx{
		Anonymous:      true,
		TTL:            ttlDuration,
		ConnectTimeout: connectTimeout,
		ConnSize:       3,
	})
	defer serverCleanup()

	t.Run("ConnPool", func(t *testing.T) {
		t.Run("KickSameKey", func(t *testing.T) {
			defer resetChans()
			client1, cleanup1, _ := setupTestClient(t, nil)
			client2, cleanup2, _ := setupTestClient(t, nil)
			defer cleanup1()
			defer cleanup2()

			connectClient(t, client1, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})
			connectClient(t, client2, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})

			expectNoSignal(t, chans.DisconnedChan, messageWaitTimeout, "user should not disconnect in anonymous mode")
		})

		t.Run("ClientClose", func(t *testing.T) {
			defer resetChans()
			client, cleanup, _ := setupTestClient(t, nil)
			defer cleanup()

			conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})

			conn.Close()
			expectSignal(t, chans.DisconnedChan, messageWaitTimeout, "disconnected signal expected")
		})

		t.Run("TTL", func(t *testing.T) {
			defer resetChans()
			client, cleanup, _ := setupTestClient(t, nil)
			defer cleanup()

			connectClient(t, client, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})

			expectSignal(t, chans.DisconnedChan, ttlDuration+connectTimeout+200*time.Millisecond, "connection expired")
		})
	})

	t.Run("Message", func(t *testing.T) {
		defer resetChans()

		client, cleanup, cchans := setupTestClient(t, nil)
		defer cleanup()

		conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
			"Sec-WebSocket-Protocol": []string{"json"},
		})

		var targetKey string
		serverCtx.WebsocketConnPool.Range(func(item *ttlcache.Item[string, *WsConnContext]) bool {
			targetKey = item.Key()
			return true
		})

		t.Run("ClientSend", func(t *testing.T) {
			defer resetChans()
			testMsg := []byte("hello server")
			if err := conn.SendWebsocketMessage(testMsg); err != nil {
				t.Fatalf("send message failed: %v", err)
			}
			expectMessage(t, chans.MessageChan, testMsg, messageWaitTimeout)
		})

		t.Run("ServerSend", func(t *testing.T) {
			defer resetChans()
			testMsg := []byte("hello client")
			serverConnItem := serverCtx.WebsocketConnPool.Get(targetKey)
			if serverConnItem == nil {
				t.Fatal("server connection not found")
			}

			serverConn := serverConnItem.Value()
			if err := serverConn.SendWebsocketMessage(testMsg); err != nil {
				t.Fatalf("send message failed: %v", err)
			}
			expectMessage(t, cchans.MessageChan, testMsg, messageWaitTimeout)
		})

		t.Run("Broadcast", func(t *testing.T) {
			defer resetChans()
			client2, cleanup2, cchans2 := setupTestClient(t, nil)
			defer cleanup2()

			connectClient(t, client2, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})

			testMsg := []byte("broadcast message")
			serverCtx.BroadcastToWebSocket(testMsg, "")

			expectMessage(t, cchans.MessageChan, testMsg, messageWaitTimeout)
			expectMessage(t, cchans2.MessageChan, testMsg, messageWaitTimeout)
		})
	})

	t.Run("Config", func(t *testing.T) {
		t.Run("MaxSize", func(t *testing.T) {
			defer resetChans()
			c1, cleanup1, c1chans := setupTestClient(t, nil)
			c2, cleanup2, _ := setupTestClient(t, nil)
			c3, cleanup3, _ := setupTestClient(t, nil)
			c4, cleanup4, _ := setupTestClient(t, nil)
			defer cleanup1()
			defer cleanup2()
			defer cleanup3()
			defer cleanup4()

			connectClient(t, c1, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})
			connectClient(t, c2, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})
			connectClient(t, c3, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})
			connectClient(t, c4, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})

			if serverCtx.WebsocketConnPool.Len() != 3 {
				t.Fatalf("pool size should be 3, got %d", serverCtx.WebsocketConnPool.Len())
			}

			expectSignal(t, c1chans.DisconnedChan, time.Second, "first client should be kicked when pool is full")
		})
	})

	t.Run("Store", func(t *testing.T) {
		defer resetChans()

		client, cleanup, _ := setupTestClient(t, nil)
		defer cleanup()

		conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
			"Sec-WebSocket-Protocol": []string{"json"},
		})

		var targetKey string
		serverCtx.WebsocketConnPool.Range(func(item *ttlcache.Item[string, *WsConnContext]) bool {
			targetKey = item.Key()
			return true
		})

		serverConnItem := serverCtx.WebsocketConnPool.Get(targetKey)
		if serverConnItem == nil {
			t.Fatal("server connection not found")
		}
		serverConn := serverConnItem.Value()

		t.Run("SetByConnContext", func(t *testing.T) {
			serverConn.SetStore("key1", "server_val")
			val, ok := serverConn.GetStore("key1")
			if !ok || val != "server_val" {
				t.Fatalf("store get failed: %v, %v", val, ok)
			}
		})

		t.Run("SetByContextValue", func(t *testing.T) {
			store := serverConn.Ctx.Value("mtc-store").(map[string]string)
			store["key2"] = "ctx_val"

			val, ok := serverConn.GetStore("key2")
			if !ok || val != "ctx_val" {
				t.Fatalf("store get failed: %v, %v", val, ok)
			}
		})

		t.Run("ClientStore", func(t *testing.T) {
			conn.SetStore("ckey", "cval")
			val, ok := conn.GetStore("ckey")
			if !ok || val != "cval" {
				t.Fatalf("client store get failed: %v, %v", val, ok)
			}

			cstore := conn.Ctx.Value("mtc-store").(map[string]string)
			if cstore["ckey"] != "cval" {
				t.Fatalf("store in context check failed")
			}
		})
	})

	t.Run("ContextCancel", func(t *testing.T) {
		defer resetChans()
		c1, cleanup1, c1chans := setupTestClient(t, nil)
		c2, cleanup2, c2chans := setupTestClient(t, nil)
		defer cleanup1()
		defer cleanup2()

		connectClient(t, c1, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})
		connectClient(t, c2, wsURL(httpServer.URL), http.Header{"Sec-WebSocket-Protocol": []string{"json"}})

		serverCtx.Stop()

		expectSignal(t, c1chans.DisconnedChan, time.Second, "c1 should disconnect when context cancelled")
		expectSignal(t, c2chans.DisconnedChan, time.Second, "c2 should disconnect when context cancelled")
	})
}

func TestWebsocketAuthServer(t *testing.T) {
	authWsServer, httpServer, serverCleanup, resetChans, chans := setupTestServer(t, &WsCoreCtx{
		TTL:            ttlDuration,
		ConnectTimeout: connectTimeout,
		ConnSize:       3,
	})
	defer serverCleanup()

	t.Run("ConnPool", func(t *testing.T) {
		t.Run("RejectAnonymous", func(t *testing.T) {
			defer resetChans()
			client, cleanup, _ := setupTestClient(t, nil)
			defer cleanup()

			_, err := client.WebsocketClient(wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			}, nil)
			if err == nil {
				t.Fatal("anonymous should be rejected when Anonymous=false")
			}
		})

		t.Run("KickDuplicateKey", func(t *testing.T) {
			defer resetChans()
			c1, cleanup1, c1chans := setupTestClient(t, nil)
			c2, cleanup2, c2chans := setupTestClient(t, nil)
			defer cleanup1()
			defer cleanup2()

			conn1 := connectClient(t, c1, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"user:1"},
			})

			conn2 := connectClient(t, c2, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"user:1"},
			})

			kicked := false
			select {
			case <-c1chans.DisconnedChan:
				kicked = true
				time.Sleep(50 * time.Millisecond)
				reason, ok := conn1.Store["disconnect_reason"]
				if !ok || reason != "kick" {
					t.Fatalf("c1 disconnect reason incorrect: %v (exists: %v)", reason, ok)
				}
			case <-c2chans.DisconnedChan:
				kicked = true
				time.Sleep(50 * time.Millisecond)
				reason, ok := conn2.Store["disconnect_reason"]
				if !ok || reason != "kick" {
					t.Fatalf("c2 disconnect reason incorrect: %v (exists: %v)", reason, ok)
				}
			case <-time.After(time.Second):
				t.Fatal("expected one connection to be kicked")
			}
			if !kicked {
				t.Fatal("no connection disconnected")
			}
		})

		t.Run("TTL", func(t *testing.T) {
			defer resetChans()
			client, cleanup, _ := setupTestClient(t, nil)
			defer cleanup()

			connectClient(t, client, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"user:2"},
			})

			expectSignal(t, chans.DisconnedChan, ttlDuration+connectTimeout+200*time.Millisecond, "connection expired")
		})

		t.Run("ConcurrentDuplicateLogin", func(t *testing.T) {
			defer resetChans()

			authWsServer.OnConnected = func(ctx *WsConnContext) error {
				time.Sleep(50 * time.Millisecond)
				chans.ConnedChan <- struct{}{}
				return nil
			}

			c1, cleanup1, _ := setupTestClient(t, nil)
			c2, cleanup2, _ := setupTestClient(t, nil)
			defer cleanup1()
			defer cleanup2()

			connectClient(t, c1, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"user:3"},
			})

			_, err := c2.WebsocketClient(wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"user:3"},
			}, nil)
			if err == nil {
				t.Fatal("second concurrent login should be rejected")
			}
		})

		t.Run("InvalidAuth", func(t *testing.T) {
			defer resetChans()
			client, cleanup, _ := setupTestClient(t, nil)
			defer cleanup()

			_, err := client.WebsocketClient(wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
				"Authorization":          []string{"invalid"},
			}, nil)
			if err == nil {
				t.Fatal("invalid auth format should be rejected")
			}
		})
	})
}

// TODO review it...
func TestConnectTimeout(t *testing.T) {
	testTimeouts := []struct {
		name           string
		ttl            time.Duration
		connectTimeout time.Duration
		waitTime       time.Duration
	}{
		{
			name:           "ZeroConnectTimeout",
			ttl:            500 * time.Millisecond,
			connectTimeout: 0,
			waitTime:       600 * time.Millisecond,
		},
		{
			name:           "ShortConnectTimeout",
			ttl:            500 * time.Millisecond,
			connectTimeout: 100 * time.Millisecond,
			waitTime:       700 * time.Millisecond,
		},
		{
			name:           "LongConnectTimeout",
			ttl:            500 * time.Millisecond,
			connectTimeout: 500 * time.Millisecond,
			waitTime:       1100 * time.Millisecond,
		},
	}

	for _, tc := range testTimeouts {
		t.Run(tc.name, func(t *testing.T) {
			_, httpServer, serverCleanup, resetChans, _ := setupTestServer(t, &WsCoreCtx{
				Anonymous:      true,
				TTL:            tc.ttl,
				ConnectTimeout: tc.connectTimeout,
				ConnSize:       10,
			})
			defer serverCleanup()
			defer resetChans()

			client, cleanup, cchans := setupTestClient(t, nil)
			defer cleanup()

			conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
				"Sec-WebSocket-Protocol": []string{"json"},
			})

			disconnectSignalReceived := false
			select {
			case <-cchans.DisconnedChan:
				disconnectSignalReceived = true
			case <-time.After(tc.waitTime + 100*time.Millisecond):
				t.Fatalf("expected disconnection signal within %v", tc.waitTime+100*time.Millisecond)
			}

			if !disconnectSignalReceived {
				t.Fatal("disconnection signal not received")
			}

			time.Sleep(50 * time.Millisecond)
			reason, ok := conn.GetStore("disconnect_reason")
			if ok && reason != "" {
				t.Logf("disconnect_reason for %s: %s", tc.name, reason)
			}
		})
	}
}

// TODO review it...
func TestHooksNil(t *testing.T) {
	t.Run("AnonymousWithoutHooks", func(t *testing.T) {
		serverCtx, httpServer, serverCleanup, resetChans, _ := setupTestServer(t, &WsCoreCtx{
			Anonymous:      true,
			TTL:            ttlDuration,
			ConnectTimeout: connectTimeout,
		})

		serverCtx.OnConnected = nil
		serverCtx.OnDisConnected = nil
		serverCtx.OnMessage = nil

		defer serverCleanup()
		defer resetChans()

		client, cleanup, cchans := setupTestClient(t, nil)
		defer cleanup()

		conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
			"Sec-WebSocket-Protocol": []string{"json"},
		})

		msg := []byte("test message")
		if err := conn.SendWebsocketMessage(msg); err != nil {
			t.Fatalf("send message should work with nil hooks: %v", err)
		}

		conn.Close()
		expectSignal(t, cchans.DisconnedChan, messageWaitTimeout*2, "should disconnect normally with nil hooks")
	})

	t.Run("AuthWithoutHooks", func(t *testing.T) {
		serverCtx, httpServer, serverCleanup, resetChans, _ := setupTestServer(t, &WsCoreCtx{
			TTL:            ttlDuration,
			ConnectTimeout: connectTimeout,
		})

		serverCtx.OnConnected = nil
		serverCtx.OnDisConnected = nil
		serverCtx.OnMessage = nil

		defer serverCleanup()
		defer resetChans()

		client, cleanup, cchans := setupTestClient(t, nil)
		defer cleanup()

		conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
			"Sec-WebSocket-Protocol": []string{"json"},
			"Authorization":          []string{"user:test"},
		})

		msg := []byte("auth user message")
		if err := conn.SendWebsocketMessage(msg); err != nil {
			t.Fatalf("send message should work with nil hooks: %v", err)
		}

		conn.Close()
		expectSignal(t, cchans.DisconnedChan, messageWaitTimeout*2, "should disconnect normally with nil hooks")
	})

	t.Run("OnDisConnectedOnlyNil", func(t *testing.T) {
		serverCtx, httpServer, serverCleanup, resetChans, chans := setupTestServer(t, &WsCoreCtx{
			Anonymous:      true,
			TTL:            ttlDuration,
			ConnectTimeout: connectTimeout,
		})

		serverCtx.OnConnected = func(ctx *WsConnContext) error {
			chans.ConnedChan <- struct{}{}
			return nil
		}
		serverCtx.OnDisConnected = nil
		serverCtx.OnMessage = func(ctx *WsConnContext, data []byte) ([]byte, error) {
			chans.MessageChan <- data
			return nil, nil
		}

		defer serverCleanup()
		defer resetChans()

		client, cleanup, cchans := setupTestClient(t, nil)
		defer cleanup()

		conn := connectClient(t, client, wsURL(httpServer.URL), http.Header{
			"Sec-WebSocket-Protocol": []string{"json"},
		})

		expectSignal(t, chans.ConnedChan, messageWaitTimeout, "OnConnected should be called")

		conn.Close()
		expectSignal(t, cchans.DisconnedChan, messageWaitTimeout, "client should disconnect even with nil OnDisConnected")
	})
}
