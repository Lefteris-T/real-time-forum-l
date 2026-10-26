package handler

import (
	"encoding/json"
	"net/http"

	"forum/internal/model"
)

type CategoryLister interface {
	All() ([]model.Category, error)
}

type CategoryListHandler struct {
	categories CategoryLister
}

type categoryListResponse struct {
	Categories []categoryResponse `json:"categories"`
}

type categoryResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func NewCategoryListHandler(categories CategoryLister) *CategoryListHandler {
	return &CategoryListHandler{categories: categories}
}

func (h *CategoryListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeAuthJSON(w, http.StatusMethodNotAllowed, authResponse{Error: http.StatusText(http.StatusMethodNotAllowed)})
		return
	}

	categories, err := h.categories.All()
	if err != nil {
		writeAuthJSON(w, http.StatusInternalServerError, authResponse{Error: http.StatusText(http.StatusInternalServerError)})
		return
	}

	response := categoryListResponse{
		Categories: make([]categoryResponse, 0, len(categories)),
	}
	for _, category := range categories {
		response.Categories = append(response.Categories, categoryResponse{
			ID:   category.ID,
			Name: category.Name,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(response)
}
