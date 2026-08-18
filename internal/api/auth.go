package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// withAuth enforces the bearer-token API auth required on every route. The
// token comes from the required API_TOKEN env var (see internal/config),
// which fails startup if unset rather than allowing all requests silently.
func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(auth, "Bearer ")
		if !ok || subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}
