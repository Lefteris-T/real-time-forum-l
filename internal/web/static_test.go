package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticFileHandlerDisablesDirectoryListings(
	t *testing.T,
) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "style.css"),
		[]byte("body {}"),
		0644,
	); err != nil {
		t.Fatalf("write stylesheet: %v", err)
	}

	handler := NewStaticFileHandler(root)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "root directory listing",
			path:       "/",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "stylesheet remains public",
			path:       "/style.css",
			wantStatus: http.StatusOK,
			wantBody:   "body {}",
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(
				http.MethodGet,
				tt.path,
				nil,
			)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf(
					"status = %d, want %d",
					rec.Code,
					tt.wantStatus,
				)
			}

			if tt.wantBody != "" &&
				rec.Body.String() != tt.wantBody {
				t.Errorf(
					"body = %q, want %q",
					rec.Body.String(),
					tt.wantBody,
				)
			}
		})
	}
}
