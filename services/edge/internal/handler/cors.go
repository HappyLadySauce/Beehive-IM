package handler

import (
	"net/http"

	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/config"
	"github.com/HappyLadySauce/Beehive-IM/services/edge/internal/security"
)

const (
	corsAllowedMethods = "GET, POST, PATCH, DELETE, OPTIONS"
	corsAllowedHeaders = "Authorization, Content-Type, X-Device-Id, X-Debug-User-Id, X-Debug-Device-Id"
	corsMaxAge         = "600"
)

func NewCORSMiddleware(conf config.SecurityConf) func(http.Handler) http.Handler {
	checker := security.NewOriginChecker(conf.AllowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !checker.Allowed(origin) {
				writeJSONError(w, r, http.StatusForbidden, "INVALID_ORIGIN", "Origin is not allowed")
				return
			}
			writeCORSHeaders(w, origin)
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeCORSHeaders(w http.ResponseWriter, origin string) {
	header := w.Header()
	header.Add("Vary", "Origin")
	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Credentials", "true")
	header.Set("Access-Control-Allow-Methods", corsAllowedMethods)
	header.Set("Access-Control-Allow-Headers", corsAllowedHeaders)
	header.Set("Access-Control-Max-Age", corsMaxAge)
}
