package handler

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPAHandlerServesIndexHTML(t *testing.T) {
	indexPath := filepath.Join(t.TempDir(), "index.html")
	want := "<h1>Real-Time Forum</h1>"

	if err := os.WriteFile(indexPath, []byte(want), 0o600); err != nil {
		t.Fatalf("write test index: %v", err)
	}

	handler := NewSPAHandler(indexPath)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if body := strings.TrimSpace(response.Body.String()); body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}

	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("Content-Type = %q, want text/html", contentType)
	}
}
