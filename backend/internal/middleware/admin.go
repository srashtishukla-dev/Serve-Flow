package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/serveflow/serveflow/backend/internal/auth"
	"github.com/serveflow/serveflow/backend/internal/response"
	"github.com/serveflow/serveflow/backend/internal/users"
)

type userFinder interface {
	FindByID(context.Context, string) (users.User, error)
}

// RequireAdmin allows administrators only.
func RequireAdmin(finder userFinder) func(http.Handler) http.Handler {
	return RequireRoles(finder, users.RoleAdmin)
}

// RequireRoles must run after JWT. The role is read from the database on every
// request, so it never depends on a client-supplied or stale token value.
func RequireRoles(finder userFinder, roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := auth.PrincipalFromContext(r.Context())
			if !ok {
				response.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "A bearer access token is required")
				return
			}
			user, err := finder.FindByID(r.Context(), principal.UserID)
			if errors.Is(err, users.ErrNotFound) {
				response.Error(w, http.StatusUnauthorized, "INVALID_TOKEN", "The access token is invalid or expired")
				return
			}
			if err != nil {
				response.Error(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Unable to verify permissions")
				return
			}
			allowed := len(roles) == 0
			for _, role := range roles {
				allowed = allowed || user.Role == role
			}
			if !allowed || user.OrganizationID != principal.OrganizationID {
				response.Error(w, http.StatusForbidden, "FORBIDDEN", "You do not have permission to perform this action")
				return
			}
			next.ServeHTTP(w, r.WithContext(users.WithUser(r.Context(), user)))
		})
	}
}
