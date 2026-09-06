package public

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ochidoma/platform/internal/repository"
)

type DecreeHandler struct {
	repo *repository.DecreeRepo
}

func NewDecreeHandler(repo *repository.DecreeRepo) *DecreeHandler {
	return &DecreeHandler{repo: repo}
}

func (h *DecreeHandler) List(w http.ResponseWriter, r *http.Request) {
	decrees, err := h.repo.ListPublished(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load decrees")
		return
	}
	writeJSON(w, http.StatusOK, decrees)
}

func (h *DecreeHandler) GetByNumber(w http.ResponseWriter, r *http.Request) {
	decreeNumber := chi.URLParam(r, "decreeNumber")
	decree, err := h.repo.GetByNumber(r.Context(), decreeNumber)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "decree not found")
		return
	}
	writeJSON(w, http.StatusOK, decree)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
