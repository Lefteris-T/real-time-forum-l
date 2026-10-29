package handler

import (
	"bytes"
	"encoding/json"
	"errors"
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
	if h.clients[client.userID] == nil {
		h.clients[client.userID] = make(map[*webSocketClient]struct{})
	}
	h.clients[client.userID][client] = struct{}{}
	h.clientsWG.Add(1)
	return true
}

func (h *WebSocketHub) unregister(client *webSocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	userClients := h.clients[client.userID]
	delete(userClients, client)
	if len(userClients) == 0 {
		delete(h.clients, client.userID)
	}
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

type webSocketClient struct {
	userID   int64
	conn     *websocket.Conn
	hub      *WebSocketHub
	send     chan []byte
	done     chan struct{}
	stopOnce sync.Once
}

func newWebSocketClient(userID int64, conn *websocket.Conn, hub *WebSocketHub) *webSocketClient {
	return &webSocketClient{
		userID: userID,
		conn:   conn,
		hub:    hub,
		send:   make(chan []byte, websocketSendQueueSize),
		done:   make(chan struct{}),
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
