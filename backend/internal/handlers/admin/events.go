package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ochidoma/platform/internal/audit"
	"github.com/ochidoma/platform/internal/repository"
)

type EventHandler struct {
	repo *repository.EventRepo
}

func NewEventHandler(repo *repository.EventRepo) *EventHandler {
	return &EventHandler{repo: repo}
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.AdminList(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load events")
		return
	}
	writeJSON(w, http.StatusOK, events)
}

type eventRequest struct {
	Name        string     `json:"name"`
	Slug        string     `json:"slug"`
	Description *string    `json:"description"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      *time.Time `json:"ends_at"`
	Location    *string    `json:"location"`
	Organizer   *string    `json:"organizer"`
	Category    *string    `json:"category"`
}

func toEventInput(req eventRequest) repository.EventInput {
	return repository.EventInput{
		Name: req.Name, Slug: req.Slug, Description: req.Description, StartsAt: req.StartsAt,
		EndsAt: req.EndsAt, Location: req.Location, Organizer: req.Organizer, Category: req.Category,
	}
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" || req.StartsAt.IsZero() {
		writeError(w, http.StatusBadRequest, "name, slug, and starts_at are required")
		return
	}

	event, err := h.repo.AdminCreate(r.Context(), toEventInput(req), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create event")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionEventCreated,
		EntityType: "event", EntityID: event.ID, NewState: event, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, event)
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	before, err := h.repo.AdminGetByID(r.Context(), id)
	if err != nil || before == nil {
		writeError(w, http.StatusNotFound, "event not found")
		return
	}

	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	after, err := h.repo.AdminUpdate(r.Context(), id, toEventInput(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update event")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_UPDATED_EVENT",
		EntityType: "event", EntityID: id, PreviousState: before, NewState: after, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, after)
}

func (h *EventHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"` // upcoming | ongoing | completed
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	event, err := h.repo.AdminSetStatus(r.Context(), id, req.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "EVENT_STATUS_CHANGED",
		EntityType: "event", EntityID: id, NewState: event, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, event)
}

func (h *EventHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.AdminDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete event")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_DELETED_EVENT",
		EntityType: "event", EntityID: id, IPAddress: r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}
