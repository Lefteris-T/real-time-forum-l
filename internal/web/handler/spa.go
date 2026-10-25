package handler

import (
	"net/http"
)

type SPAHandler struct {
	indexPath string
}

func NewSPAHandler(indexPath string) *SPAHandler {
	return &SPAHandler{
		indexPath: indexPath,
	}
}

func (h *SPAHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w, r, h.indexPath)
}