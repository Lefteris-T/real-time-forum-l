package handler

import (
	"net/http"
	"net/url"
	"strings"

	"forum/internal/web/middleware"
	"github.com/gorilla/websocket"
)

const websocketReadLimit = 16 << 10

// WebSocketHandler authenticates, upgrades, and registers browser connections
// with the shared hub.
type WebSocketHandler struct {
	upgrader websocket.Upgrader
	hub      *WebSocketHub
}

func NewWebSocketHandler(hub *WebSocketHub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return sameOriginWebSocketRequest(r)
			},
		},
	}
}

func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	user, ok := middleware.CurrentUser(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}
	if !sameOriginWebSocketRequest(r) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if h.hub == nil {
		http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		// The upgrader writes an HTTP error response when the handshake fails.
		return
	}
	client := newWebSocketClient(user.ID, conn, h.hub)
	if !h.hub.register(client) {
		client.stop()
		return
	}
	client.serve()
}

// sameOriginWebSocketRequest accepts only a single serialized browser Origin
// matching the direct request's scheme and authority. Forwarding headers are
// deliberately ignored because this server has no trusted-proxy boundary.
func sameOriginWebSocketRequest(r *http.Request) bool {
	origins := r.Header.Values("Origin")
	if len(origins) != 1 || strings.TrimSpace(origins[0]) == "" {
		return false
	}
	origin, err := url.Parse(origins[0])
	if err != nil || origin.Opaque != "" || origin.User != nil || origin.Host == "" ||
		origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return false
	}

	wantScheme := "http"
	if r.TLS != nil {
		wantScheme = "https"
	}
	return strings.EqualFold(origin.Scheme, wantScheme) &&
		strings.EqualFold(origin.Host, r.Host)
}
