package view

import (
	"path/filepath"
	"testing"
)

func TestRealTemplatesLoad(t *testing.T) {
	renderer, err := NewRenderer(
		filepath.Join("..", "..", "..", "templates"),
	)
	if err != nil {
		t.Fatalf("NewRenderer() error: %v", err)
	}

	if renderer == nil {
		t.Fatal("renderer is nil")
	}
}
