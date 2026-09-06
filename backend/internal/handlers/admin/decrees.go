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

type DecreeHandler struct {
	repo *repository.DecreeRepo
}

func NewDecreeHandler(repo *repository.DecreeRepo) *DecreeHandler {
	return &DecreeHandler{repo: repo}
}

func (h *DecreeHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.AdminListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load decrees")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *DecreeHandler) Get(w http.ResponseWriter, r *http.Request) {
	versionID, err := parseID(w, r)
	if err != nil {
		return
	}
	v, err := h.repo.AdminGetVersion(r.Context(), versionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load decree version")
		return
	}
	if v == nil {
		writeError(w, http.StatusNotFound, "decree version not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

type decreeRequest struct {
	DecreeNumber     string `json:"decree_number"`
	Title            string `json:"title"`
	FullText         string `json:"full_text"`
	IssuingAuthority string `json:"issuing_authority"`
}

// Create makes a new decree plus its first draft version (1.0).
// palace_editor/super_admin (route-gated in main.go).
func (h *DecreeHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	var req decreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DecreeNumber == "" || req.Title == "" || req.FullText == "" || req.IssuingAuthority == "" {
		writeError(w, http.StatusBadRequest, "decree_number, title, full_text, and issuing_authority are required")
		return
	}

	v, err := h.repo.AdminCreate(r.Context(), repository.DecreeInput{
		DecreeNumber: req.DecreeNumber, Title: req.Title, FullText: req.FullText,
		IssuingAuthority: req.IssuingAuthority,
	}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create decree — decree_number may already be in use")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreeCreated,
		EntityType: "decree_version", EntityID: v.ID, NewState: v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, v)
}

// Submit: draft -> in_review. palace_editor/super_admin.
func (h *DecreeHandler) Submit(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	v, err := h.repo.AdminSubmit(r.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "decree version must be 'draft' to submit for review")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to submit decree")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreeSubmitted,
		EntityType: "decree_version", EntityID: id, NewState: v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, v)
}

// Approve: in_review -> approved. palace_publisher/super_admin only (route-gated in main.go).
// A palace_editor cannot reach this handler at all — RequireRole blocks it before the request arrives here.
func (h *DecreeHandler) Approve(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	v, err := h.repo.AdminApprove(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "decree version must be 'in_review' to approve")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to approve decree")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreeApproved,
		EntityType: "decree_version", EntityID: id, NewState: v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, v)
}

// Publish: approved -> published. palace_publisher/super_admin only.
// After this, the DB trigger makes the version's content immutable —
// only a correction (new version) can change it from here.
func (h *DecreeHandler) Publish(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	v, err := h.repo.AdminPublish(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "decree version must be 'approved' to publish")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to publish decree")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreePublished,
		EntityType: "decree_version", EntityID: id, NewState: v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, v)
}

// Archive: published (or any) -> archived. palace_publisher/super_admin only.
func (h *DecreeHandler) Archive(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := parseID(w, r)
	if err != nil {
		return
	}
	v, err := h.repo.AdminArchive(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to archive decree")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreeArchived,
		EntityType: "decree_version", EntityID: id, NewState: v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, v)
}

// Correct creates a brand-new draft version that supersedes an already-
// published one, rather than editing it. The original stays untouched and
// visible. palace_editor/super_admin — the correction still has to go
// through submit -> approve -> publish like any other version.
func (h *DecreeHandler) Correct(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	originalID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	var req decreeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.FullText == "" || req.IssuingAuthority == "" {
		writeError(w, http.StatusBadRequest, "title, full_text, and issuing_authority are required")
		return
	}

	v, err := h.repo.AdminCreateCorrection(r.Context(), originalID, repository.DecreeInput{
		Title: req.Title, FullText: req.FullText, IssuingAuthority: req.IssuingAuthority,
	}, userID)
	if err != nil {
		if errors.Is(err, repository.ErrInvalidTransition) {
			writeError(w, http.StatusConflict, "can only correct a version that is currently published")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create correction")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionDecreeCorrected,
		EntityType: "decree_version", EntityID: v.ID,
		PreviousState: map[string]string{"supersedes_version_id": originalID.String()},
		NewState:      v, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, v)
}
