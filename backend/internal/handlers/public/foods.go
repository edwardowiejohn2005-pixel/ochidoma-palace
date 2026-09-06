package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ochidoma/platform/internal/repository"
)

type FoodHandler struct {
	repo *repository.FoodRepo
}

func NewFoodHandler(repo *repository.FoodRepo) *FoodHandler {
	return &FoodHandler{repo: repo}
}

func (h *FoodHandler) List(w http.ResponseWriter, r *http.Request) {
	foods, err := h.repo.ListPublished(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load foods")
		return
	}
	writeJSON(w, http.StatusOK, foods)
}

func (h *FoodHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	food, err := h.repo.GetBySlug(r.Context(), slug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load food")
		return
	}
	if food == nil {
		writeJSONError(w, http.StatusNotFound, "food not found")
		return
	}
	writeJSON(w, http.StatusOK, food)
}
