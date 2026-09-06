package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ochidoma/platform/internal/audit"
	"github.com/ochidoma/platform/internal/repository"
)

type ArticleHandler struct {
	repo *repository.ArticleRepo
}

func NewArticleHandler(repo *repository.ArticleRepo) *ArticleHandler {
	return &ArticleHandler{repo: repo}
}

func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	articles, err := h.repo.AdminList(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load articles")
		return
	}
	writeJSON(w, http.StatusOK, articles)
}

type articleRequest struct {
	Section     string  `json:"section"`
	CategoryID  *int    `json:"category_id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	Description *string `json:"description"`
	Body        string  `json:"body"`
	Sources     *string `json:"sources"`
}

func (h *ArticleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	var req articleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Title == "" || req.Slug == "" || req.Body == "" || req.Section == "" {
		writeError(w, http.StatusBadRequest, "section, title, slug, and body are required")
		return
	}

	article, err := h.repo.AdminCreate(r.Context(), repository.ArticleInput{
		Section: req.Section, CategoryID: req.CategoryID, Title: req.Title,
		Slug: req.Slug, Description: req.Description, Body: req.Body, Sources: req.Sources,
	}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create article")
		return
	}

	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionArticleCreated,
		EntityType: "article", EntityID: article.ID, NewState: article, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, article)
}

func (h *ArticleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	before, err := h.repo.AdminGetByID(r.Context(), id)
	if err != nil || before == nil {
		writeError(w, http.StatusNotFound, "article not found")
		return
	}

	var req articleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	after, err := h.repo.AdminUpdate(r.Context(), id, repository.ArticleInput{
		Section: req.Section, CategoryID: req.CategoryID, Title: req.Title,
		Slug: req.Slug, Description: req.Description, Body: req.Body, Sources: req.Sources,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update article")
		return
	}

	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: audit.ActionArticleUpdated,
		EntityType: "article", EntityID: id, PreviousState: before, NewState: after, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, after)
}

// SetStatus transitions status (draft/in_review/approved/published/archived).
// The route wiring in main.go restricts which roles may call which target
// status — this handler just performs the transition it's told to.
func (h *ArticleHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	article, err := h.repo.AdminSetStatus(r.Context(), id, req.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status")
		return
	}

	action := audit.ActionArticleUpdated
	if req.Status == "published" {
		action = audit.ActionArticlePublished
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: action,
		EntityType: "article", EntityID: id, NewState: article, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, article)
}

func (h *ArticleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.AdminDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete article")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_DELETED_ARTICLE",
		EntityType: "article", EntityID: id, IPAddress: r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}
