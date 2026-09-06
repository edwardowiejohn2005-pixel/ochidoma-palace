package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ochidoma/platform/internal/repository"
)

type EventHandler struct {
	repo *repository.EventRepo
}

func NewEventHandler(repo *repository.EventRepo) *EventHandler {
	return &EventHandler{repo: repo}
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status") // upcoming | ongoing | completed | "" for all
	events, err := h.repo.List(r.Context(), status)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load events")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *EventHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	event, err := h.repo.GetBySlug(r.Context(), slug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load event")
		return
	}
	if event == nil {
		writeJSONError(w, http.StatusNotFound, "event not found")
		return
	}
	writeJSON(w, http.StatusOK, event)
}
