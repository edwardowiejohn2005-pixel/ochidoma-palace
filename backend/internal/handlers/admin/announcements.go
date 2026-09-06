package admin

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ochidoma/platform/internal/audit"
	"github.com/ochidoma/platform/internal/repository"
)

type AnnouncementHandler struct {
	repo *repository.AnnouncementRepo
}

func NewAnnouncementHandler(repo *repository.AnnouncementRepo) *AnnouncementHandler {
	return &AnnouncementHandler{repo: repo}
}

func (h *AnnouncementHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.repo.AdminList(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load announcements")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

type announcementRequest struct {
	Title    string  `json:"title"`
	Category *string `json:"category"`
	Content  string  `json:"content"`
}

func (h *AnnouncementHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	var req announcementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	a, err := h.repo.AdminCreate(r.Context(), repository.AnnouncementInput{
		Title: req.Title, Category: req.Category, Content: req.Content,
	}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create announcement")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionAnnouncementCreated,
		EntityType: "announcement", EntityID: a.ID, NewState: a, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, a)
}

func (h *AnnouncementHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	before, err := h.repo.AdminGetByID(r.Context(), id)
	if err != nil || before == nil {
		writeError(w, http.StatusNotFound, "announcement not found")
		return
	}

	var req announcementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	after, err := h.repo.AdminUpdate(r.Context(), id, repository.AnnouncementInput{
		Title: req.Title, Category: req.Category, Content: req.Content,
	})
	if err != nil {
		// The DB trigger rejects edits to already-published rows — surface
		// that as a clear conflict rather than a generic 500.
		writeError(w, http.StatusConflict, "cannot edit: this announcement is already published — create a correction instead")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ANNOUNCEMENT_UPDATED",
		EntityType: "announcement", EntityID: id, PreviousState: before, NewState: after, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, after)
}

// Submit: draft -> in_review. Any palace_editor/super_admin (route-gated in main.go).
func (h *AnnouncementHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	a, err := h.repo.AdminSubmit(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to submit announcement")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ANNOUNCEMENT_SUBMITTED",
		EntityType: "announcement", EntityID: id, NewState: a, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, a)
}

// Approve: in_review -> approved. palace_publisher/super_admin only (route-gated in main.go).
func (h *AnnouncementHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	a, err := h.repo.AdminApprove(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve announcement")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ANNOUNCEMENT_APPROVED",
		EntityType: "announcement", EntityID: id, NewState: a, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, a)
}

// Archive: any status -> archived. palace_publisher/super_admin only (route-gated in main.go).
func (h *AnnouncementHandler) Archive(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	a, err := h.repo.AdminArchive(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive announcement")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ANNOUNCEMENT_ARCHIVED",
		EntityType: "announcement", EntityID: id, NewState: a, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, a)
}

// Publish: approved -> published, assigns reference number. palace_publisher/super_admin only.
func (h *AnnouncementHandler) Publish(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}

	a, err := h.repo.AdminPublish(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "announcement must be 'approved' before it can be published")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to publish announcement")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionAnnouncementPublish,
		EntityType: "announcement", EntityID: id, NewState: a, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, a)
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
	}
	return id, err
}
