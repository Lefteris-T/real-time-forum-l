package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"forum/internal/model"
	"forum/internal/web/middleware"
	"github.com/gorilla/websocket"
)

func TestWebSocketHandlerRejectsGuestAndInvalidOrigins(t *testing.T) {
	handler := NewWebSocketHandler(NewWebSocketHub())
	guest := httptest.NewRecorder()
	handler.ServeHTTP(guest, httptest.NewRequest(http.MethodGet, "/ws", nil))
	if guest.Code != http.StatusUnauthorized {
		t.Fatalf("guest status = %d, want %d", guest.Code, http.StatusUnauthorized)
	}

	for _, origin := range []string{"", "https://forum.example", "http://other.example", "http://forum.example/path"} {
		req := httptest.NewRequest(http.MethodGet, "http://forum.example/ws", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 4}))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("origin %q status = %d, want %d", origin, recorder.Code, http.StatusForbidden)
		}
	}
}

func TestWebSocketHubBroadcastsToMultipleConnectionsAndShutsDown(t *testing.T) {
	hub := NewWebSocketHub()
	server := authenticatedWebSocketServer(hub, 42)
	t.Cleanup(func() {
		hub.Shutdown()
		server.Close()
	})

	first := dialForumWebSocket(t, server)
	second := dialForumWebSocket(t, server)
	waitForWebSocketClientCount(t, hub, 42, 2)

	const eventCount = 12
	var publishers sync.WaitGroup
	for i := 0; i < eventCount; i++ {
		publishers.Add(1)
		go func(sequence int) {
			defer publishers.Done()
			event := WebSocketEvent{
				Type:    "test.event",
				Payload: json.RawMessage(fmt.Sprintf(`{"sequence":%d}`, sequence)),
			}
			if err := hub.Publish(42, event); err != nil {
				t.Errorf("Publish() error: %v", err)
			}
		}(i)
	}
	publishers.Wait()
	assertWebSocketEvents(t, first, eventCount)
	assertWebSocketEvents(t, second, eventCount)

	_ = first.Close()
	waitForWebSocketClientCount(t, hub, 42, 1)
	if err := hub.Publish(42, WebSocketEvent{Type: "still.connected"}); err != nil {
		t.Fatalf("Publish() after one client disconnected: %v", err)
	}
	assertWebSocketEvents(t, second, 1)

	shutdownDone := make(chan struct{})
	go func() {
		hub.Shutdown()
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(3 * time.Second):
		t.Fatal("hub shutdown did not wait for and close its client")
	}
	if _, _, err := second.ReadMessage(); err == nil {
		t.Fatal("connection remained open after hub shutdown")
	}
	_ = second.Close()
}

func TestWebSocketHandlerClosesUnsupportedApplicationFrames(t *testing.T) {
	hub := NewWebSocketHub()
	server := authenticatedWebSocketServer(hub, 7)
	t.Cleanup(func() {
		hub.Shutdown()
		server.Close()
	})
	conn := dialForumWebSocket(t, server)
	defer conn.Close()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"client.command"}`)); err != nil {
		t.Fatalf("write application frame: %v", err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, err := conn.ReadMessage()
	if closeErr, ok := err.(*websocket.CloseError); !ok || closeErr.Code != websocket.CloseUnsupportedData {
		t.Fatalf("read error = %#v, want close code %d", err, websocket.CloseUnsupportedData)
	}
}

func TestWebSocketClientAnswersPingAndRejectsOversizedFrames(t *testing.T) {
	hub := NewWebSocketHub()
	server := authenticatedWebSocketServer(hub, 11)
	t.Cleanup(func() {
		hub.Shutdown()
		server.Close()
	})
	conn := dialForumWebSocket(t, server)
	defer conn.Close()
	pongReceived := make(chan struct{}, 1)
	readError := make(chan error, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pongReceived <- struct{}{}:
		default:
		}
		return nil
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				readError <- err
				return
			}
		}
	}()
	_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteControl(websocket.PingMessage, []byte("ping check"), time.Now().Add(time.Second)); err != nil {
		t.Fatalf("send ping: %v", err)
	}
	select {
	case <-pongReceived:
	case <-time.After(3 * time.Second):
		t.Fatal("server did not answer the ping with a pong")
	}
	if err := conn.WriteMessage(websocket.TextMessage, make([]byte, websocketReadLimit+1)); err != nil {
		t.Fatalf("send oversized frame: %v", err)
	}
	select {
	case err := <-readError:
		closeErr, ok := err.(*websocket.CloseError)
		if !ok || closeErr.Code != websocket.CloseMessageTooBig {
			t.Fatalf("read error = %#v, want close code %d", err, websocket.CloseMessageTooBig)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not close the oversized frame")
	}
}

func authenticatedWebSocketServer(hub *WebSocketHub, userID int64) *httptest.Server {
	handler := NewWebSocketHandler(hub)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(middleware.ContextWithUser(r.Context(), model.User{ID: userID}))
		handler.ServeHTTP(w, r)
	}))
}

func dialForumWebSocket(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	conn, response, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{server.URL}})
	if err != nil {
		if response != nil {
			t.Fatalf("dial websocket: %v (HTTP %d)", err, response.StatusCode)
		}
		t.Fatalf("dial websocket: %v", err)
	}
	return conn
}

func waitForWebSocketClientCount(t *testing.T, hub *WebSocketHub, userID int64, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		got := len(hub.clients[userID])
		hub.mu.Unlock()
		if got == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("client count for user %d did not become %d", userID, want)
}

func assertWebSocketEvents(t *testing.T, conn *websocket.Conn, count int) {
	t.Helper()
	seen := make(map[int]bool, count)
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < count; i++ {
		messageType, frame, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read event %d/%d: %v", i+1, count, err)
		}
		if messageType != websocket.TextMessage {
			t.Fatalf("event frame type = %d, want text", messageType)
		}
		var event struct {
			Type    string `json:"type"`
			Payload struct {
				Sequence int `json:"sequence"`
			} `json:"payload"`
		}
		if err := json.Unmarshal(frame, &event); err != nil {
			t.Fatalf("decode event: %v", err)
		}
		if event.Type == "test.event" {
			if seen[event.Payload.Sequence] {
				t.Fatalf("duplicate event sequence %d", event.Payload.Sequence)
			}
			seen[event.Payload.Sequence] = true
		}
	}
	if count > 1 && len(seen) != count {
		t.Fatalf("received %d unique events, want %d", len(seen), count)
	}
}
