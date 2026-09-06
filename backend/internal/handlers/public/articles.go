package public

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/ochidoma/platform/internal/repository"
)

type ArticleHandler struct {
	repo *repository.ArticleRepo
}

func NewArticleHandler(repo *repository.ArticleRepo) *ArticleHandler {
	return &ArticleHandler{repo: repo}
}

func (h *ArticleHandler) List(w http.ResponseWriter, r *http.Request) {
	section := r.URL.Query().Get("section")
	var categoryID int
	if v := r.URL.Query().Get("category_id"); v != "" {
		categoryID, _ = strconv.Atoi(v) // invalid/absent just means "no filter"
	}

	articles, err := h.repo.ListPublished(r.Context(), section, categoryID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load articles")
		return
	}
	writeJSON(w, http.StatusOK, articles)
}

func (h *ArticleHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	article, err := h.repo.GetBySlug(r.Context(), slug)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to load article")
		return
	}
	if article == nil {
		writeJSONError(w, http.StatusNotFound, "article not found")
		return
	}
	writeJSON(w, http.StatusOK, article)
}
