package api

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/krau/SaveAny-Bot/config"
)

// tokenContextKey 用于在 context 中存储 token
type tokenContextKey struct{}

// AuthMiddleware 返回认证中间件
func AuthMiddleware() func(http.Handler) http.Handler {
	return TokenAuthMiddleware(func() string {
		return config.C().API.Token
	})
}

func TokenAuthMiddleware(tokenProvider func() string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicConfigWebPath(r) {
				next.ServeHTTP(w, r)
				return
			}

			token := tokenProvider()
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			requestToken := getRequestToken(r)
			if requestToken == "" {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "missing authorization header")
				return
			}

			if subtle.ConstantTimeCompare([]byte(requestToken), []byte(token)) != 1 {
				WriteError(w, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}

			ctx := context.WithValue(r.Context(), tokenContextKey{}, requestToken)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func getRequestToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			return ""
		}
		return parts[1]
	}
	if token := r.Header.Get("X-API-Token"); token != "" {
		return token
	}
	if cookie, err := r.Cookie("saveany_api_token"); err == nil {
		return cookie.Value
	}
	return r.URL.Query().Get("token")
}

func isPublicConfigWebPath(r *http.Request) bool {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return false
	}
	return r.URL.Path == "/config" || strings.HasPrefix(r.URL.Path, "/config/")
}
