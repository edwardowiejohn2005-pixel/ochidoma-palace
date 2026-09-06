package admin

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	appmiddleware "github.com/ochidoma/platform/internal/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// currentUser pulls the authenticated user's ID out of request context.
// Only ever nil/false if RequireAuth wasn't run first, which would be a
// routing bug, not a normal runtime condition.
func currentUser(r *http.Request) (uuid.UUID, bool) {
	return appmiddleware.UserIDFromContext(r.Context())
}
