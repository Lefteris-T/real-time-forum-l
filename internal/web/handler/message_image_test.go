package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"forum/internal/model"
	"forum/internal/repository"
	"forum/internal/web/middleware"
)

type messageImageReaderFake struct {
	messageID, userID int64
	image             model.MessageImage
	err               error
}

func (f *messageImageReaderFake) ImageForUser(messageID, userID int64) (model.MessageImage, error) {
	f.messageID, f.userID = messageID, userID
	return f.image, f.err
}

func TestMessageImageHandlerReturnsPrivateImageWithSafeHeaders(t *testing.T) {
	content := []byte("fake png bytes")
	reader := &messageImageReaderFake{image: model.MessageImage{MIMEType: "image/png", Data: content}}
	req := httptest.NewRequest(http.MethodGet, "/api/messages/31/image", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 8}))
	recorder := httptest.NewRecorder()
	NewMessageImageHandler(reader).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK || reader.messageID != 31 || reader.userID != 8 || !bytes.Equal(recorder.Body.Bytes(), content) {
		t.Fatalf("image response status/args/body = %d/%d/%d/%q", recorder.Code, reader.messageID, reader.userID, recorder.Body.String())
	}
	if recorder.Header().Get("Content-Type") != "image/png" || recorder.Header().Get("X-Content-Type-Options") != "nosniff" || recorder.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unsafe image headers: %v", recorder.Header())
	}
}

func TestMessageImageHandlerDoesNotExposeNonparticipantImages(t *testing.T) {
	reader := &messageImageReaderFake{err: repository.ErrMessageImageNotFound}
	req := httptest.NewRequest(http.MethodGet, "/api/messages/31/image", nil)
	req = req.WithContext(middleware.ContextWithUser(req.Context(), model.User{ID: 90}))
	recorder := httptest.NewRecorder()
	NewMessageImageHandler(reader).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("nonparticipant status = %d", recorder.Code)
	}
}
