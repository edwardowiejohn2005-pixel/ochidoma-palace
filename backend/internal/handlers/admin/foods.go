package admin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ochidoma/platform/internal/audit"
	"github.com/ochidoma/platform/internal/repository"
)

type FoodHandler struct {
	repo *repository.FoodRepo
}

func NewFoodHandler(repo *repository.FoodRepo) *FoodHandler {
	return &FoodHandler{repo: repo}
}

func (h *FoodHandler) List(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	foods, err := h.repo.AdminList(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load foods")
		return
	}
	writeJSON(w, http.StatusOK, foods)
}

type foodRequest struct {
	Name                 string  `json:"name"`
	LocalName            *string `json:"local_name"`
	Slug                 string  `json:"slug"`
	Description          *string `json:"description"`
	Ingredients          *string `json:"ingredients"`
	Preparation          *string `json:"preparation"`
	CulturalSignificance *string `json:"cultural_significance"`
	Region               *string `json:"region"`
}

func toFoodInput(req foodRequest) repository.FoodInput {
	return repository.FoodInput{
		Name: req.Name, LocalName: req.LocalName, Slug: req.Slug, Description: req.Description,
		Ingredients: req.Ingredients, Preparation: req.Preparation,
		CulturalSignificance: req.CulturalSignificance, Region: req.Region,
	}
}

func (h *FoodHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	var req foodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Slug == "" {
		writeError(w, http.StatusBadRequest, "name and slug are required")
		return
	}

	food, err := h.repo.AdminCreate(r.Context(), toFoodInput(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create food entry")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_CREATED_FOOD",
		EntityType: "food", EntityID: food.ID, NewState: food, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusCreated, food)
}

func (h *FoodHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	before, err := h.repo.AdminGetByID(r.Context(), id)
	if err != nil || before == nil {
		writeError(w, http.StatusNotFound, "food entry not found")
		return
	}

	var req foodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	after, err := h.repo.AdminUpdate(r.Context(), id, toFoodInput(req))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update food entry")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_UPDATED_FOOD",
		EntityType: "food", EntityID: id, PreviousState: before, NewState: after, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, after)
}

func (h *FoodHandler) SetStatus(w http.ResponseWriter, r *http.Request) {
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
	food, err := h.repo.AdminSetStatus(r.Context(), id, req.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "FOOD_STATUS_CHANGED",
		EntityType: "food", EntityID: id, NewState: food, IPAddress: r.RemoteAddr,
	})
	writeJSON(w, http.StatusOK, food)
}

func (h *FoodHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _ := currentUser(r)
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.repo.AdminDelete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete food entry")
		return
	}
	_ = audit.Log(r.Context(), h.repo.Pool(), audit.Entry{
		UserID: userID, Action: "ADMIN_DELETED_FOOD",
		EntityType: "food", EntityID: id, IPAddress: r.RemoteAddr,
	})
	w.WriteHeader(http.StatusNoContent)
}
