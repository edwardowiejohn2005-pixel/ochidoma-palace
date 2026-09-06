package public

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ochidoma/platform/internal/repository"
)

type AnnouncementHandler struct {
	repo *repository.AnnouncementRepo
}

func NewAnnouncementHandler(repo *repository.AnnouncementRepo) *AnnouncementHandler {
	return &AnnouncementHandler{repo: repo}
}

func (h *AnnouncementHandler) List(w http.ResponseWriter, r *http.Request) {
	announcements, err := h.repo.ListPublished(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load announcements")
		return
	}
	writeJSON(w, http.StatusOK, announcements)
}

func (h *AnnouncementHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idParam := chi.URLParam(r, "id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid announcement id")
		return
	}

	announcement, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load announcement")
		return
	}
	if announcement == nil {
		writeJSONError(w, http.StatusNotFound, "announcement not found")
		return
	}
	writeJSON(w, http.StatusOK, announcement)
}
