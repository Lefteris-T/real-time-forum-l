package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	websocketWriteWait     = 10 * time.Second
	websocketPongWait      = 60 * time.Second
	websocketPingPeriod    = 30 * time.Second
	websocketSendQueueSize = 32
)

// WebSocketEvent is the server-to-browser envelope reserved for presence and
// chat events. Browser commands continue to use HTTP endpoints.
type WebSocketEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type presenceUser struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
}

type presenceSnapshot struct {
	OnlineUsers []presenceUser `json:"online_users"`
}

type presenceChange struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	Online   bool   `json:"online"`
}

// UserConnectionCloser invalidates all live sockets for a revoked session.
type UserConnectionCloser interface {
	DisconnectUser(userID int64)
}

// WebSocketHub tracks every live connection by authenticated user. Its mutex
// protects registration and routing; each client's writer owns socket writes.
type WebSocketHub struct {
	mu        sync.Mutex
	clients   map[int64]map[*webSocketClient]struct{}
	stopped   bool
	clientsWG sync.WaitGroup
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{clients: make(map[int64]map[*webSocketClient]struct{})}
}

func (h *WebSocketHub) register(client *webSocketClient) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stopped {
		return false
	}
	firstConnection := len(h.clients[client.userID]) == 0
	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[*webSocketClient]struct{})
	}
	h.clients[client.userID][client] = struct{}{}
	h.clientsWG.Add(1)

	onlineUsers := make([]presenceUser, 0, len(h.clients))
	for userID, userClients := range h.clients {
		if userID == client.userID || len(userClients) == 0 {
			continue
		}
		for connectedClient := range userClients {
			onlineUsers = append(onlineUsers, presenceUser{UserID: userID, Username: connectedClient.username})
			break
		}
	}
	sort.Slice(onlineUsers, func(i, j int) bool { return onlineUsers[i].UserID < onlineUsers[j].UserID })
	h.enqueueLocked(client, makeWebSocketEvent("presence.snapshot", presenceSnapshot{OnlineUsers: onlineUsers}))
	if firstConnection {
		change := presenceChange{UserID: client.userID, Username: client.username, Online: true}
		h.broadcastPresenceLocked(client.userID, makeWebSocketEvent("presence.changed", change))
	}
	return true
}

func (h *WebSocketHub) unregister(client *webSocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	userClients := h.clients[client.userID]
	if _, registered := userClients[client]; !registered {
		return
	}
	delete(userClients, client)
	if len(userClients) == 0 {
		delete(h.clients, client.userID)
		if !h.stopped {
			change := presenceChange{UserID: client.userID, Username: client.username, Online: false}
			h.broadcastPresenceLocked(client.userID, makeWebSocketEvent("presence.changed", change))
		}
	}
}

func (h *WebSocketHub) broadcastPresenceLocked(excludedUserID int64, frame []byte) {
	for userID, userClients := range h.clients {
		if userID == excludedUserID {
			continue
		}
		for client := range userClients {
			h.enqueueLocked(client, frame)
		}
	}
}

func (h *WebSocketHub) enqueueLocked(client *webSocketClient, frame []byte) {
	select {
	case <-client.done:
	case client.send <- frame:
	default:
		client.stop()
	}
}

func makeWebSocketEvent(eventType string, payload any) []byte {
	encodedPayload, _ := json.Marshal(payload)
	encodedEvent, _ := json.Marshal(WebSocketEvent{Type: eventType, Payload: encodedPayload})
	return encodedEvent
}

// Publish sends one JSON event to every active connection belonging to userID.
// A slow client with a full queue is closed without delaying other clients.
func (h *WebSocketHub) Publish(userID int64, event WebSocketEvent) error {
	if userID <= 0 || event.Type == "" {
		return errors.New("invalid websocket event")
	}
	if len(event.Payload) == 0 {
		event.Payload = json.RawMessage(`{}`)
	}
	payload := bytes.TrimSpace(event.Payload)
	if !json.Valid(payload) || len(payload) == 0 || payload[0] != '{' {
		return errors.New("websocket event payload must be a JSON object")
	}
	frame, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if len(frame) > websocketReadLimit {
		return errors.New("websocket event exceeds the frame limit")
	}

	h.mu.Lock()
	clients := make([]*webSocketClient, 0, len(h.clients[userID]))
	for client := range h.clients[userID] {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	for _, client := range clients {
		select {
		case <-client.done:
		case client.send <- frame:
		default:
			client.stop()
		}
	}
	return nil
}

// PublishMessage adapts service message events to the WebSocket event envelope.
func (h *WebSocketHub) PublishMessage(userID int64, eventType string, payload json.RawMessage) error {
	return h.Publish(userID, WebSocketEvent{Type: eventType, Payload: payload})
}

// DisconnectUser closes every active tab for a user whose single server-side
// session has been replaced or revoked.
func (h *WebSocketHub) DisconnectUser(userID int64) {
	if userID <= 0 {
		return
	}
	h.mu.Lock()
	clients := make([]*webSocketClient, 0, len(h.clients[userID]))
	for client := range h.clients[userID] {
		clients = append(clients, client)
	}
	h.mu.Unlock()

	for _, client := range clients {
		client.stopWithClose(websocket.ClosePolicyViolation, "Session ended")
	}
}

// Shutdown rejects future registrations, closes all upgraded connections,
// and waits for their read and write pumps to finish.
func (h *WebSocketHub) Shutdown() {
	h.mu.Lock()
	if h.stopped {
		h.mu.Unlock()
		h.clientsWG.Wait()
		return
	}
	h.stopped = true
	clients := make([]*webSocketClient, 0)
	for _, userClients := range h.clients {
		for client := range userClients {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()

	for _, client := range clients {
		client.stop()
	}
	h.clientsWG.Wait()
}

func (h *WebSocketHub) clientFinished() {
	h.clientsWG.Done()
}

// OnlineUserIDs returns a snapshot of users with at least one active
// connection. The returned map is independent of hub state.
func (h *WebSocketHub) OnlineUserIDs() map[int64]struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	online := make(map[int64]struct{}, len(h.clients))
	for userID, clients := range h.clients {
		if len(clients) > 0 {
			online[userID] = struct{}{}
		}
	}
	return online
}

type webSocketClient struct {
	userID   int64
	username string
	conn     *websocket.Conn
	hub      *WebSocketHub
	send     chan []byte
	done     chan struct{}
	stopOnce sync.Once
}

func newWebSocketClient(userID int64, username string, conn *websocket.Conn, hub *WebSocketHub) *webSocketClient {
	return &webSocketClient{
		userID:   userID,
		username: username,
		conn:     conn,
		hub:      hub,
		send:     make(chan []byte, websocketSendQueueSize),
		done:     make(chan struct{}),
	}
}

func (c *webSocketClient) serve() {
	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		c.writePump()
	}()

	c.readPump()
	c.hub.unregister(c)
	c.stop()
	<-writerDone
	c.hub.clientFinished()
}

func (c *webSocketClient) readPump() {
	c.conn.SetReadLimit(websocketReadLimit)
	if err := c.conn.SetReadDeadline(time.Now().Add(websocketPongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(websocketPongWait))
	})
	for {
		messageType, _, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType == websocket.TextMessage || messageType == websocket.BinaryMessage {
			_ = c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "application frames are not accepted yet"),
				time.Now().Add(websocketWriteWait),
			)
			return
		}
	}
}

func (c *webSocketClient) writePump() {
	ticker := time.NewTicker(websocketPingPeriod)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(websocketWriteWait)); err != nil {
				c.stop()
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, frame); err != nil {
				c.stop()
				return
			}
		case <-ticker.C:
			if err := c.conn.SetWriteDeadline(time.Now().Add(websocketWriteWait)); err != nil {
				c.stop()
				return
			}
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.stop()
				return
			}
		}
	}
}

func (c *webSocketClient) stop() {
	c.stopOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func (c *webSocketClient) stopWithClose(code int, reason string) {
	c.stopOnce.Do(func() {
		_ = c.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(code, reason),
			time.Now().Add(websocketWriteWait),
		)
		close(c.done)
		_ = c.conn.Close()
	})
}
