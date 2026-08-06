package auth

import (
	"context"
	"net/http"
	"strings"
)

type claimsContextKey struct{}

func ClaimsFromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(claimsContextKey{}).(Claims)
	return claims, ok
}

func Require(manager *Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		token, ok := bearer(request.Header.Get("Authorization"))
		if !ok {
			unauthorized(writer)
			return
		}
		claims, err := manager.Verify(token)
		if err != nil {
			unauthorized(writer)
			return
		}
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), claimsContextKey{}, claims)))
	})
}

func bearer(value string) (string, bool) {
	parts := strings.Split(value, " ")
	returnToken := ""
	if len(parts) == 2 && parts[0] == "Bearer" && parts[1] != "" {
		returnToken = parts[1]
	}
	return returnToken, returnToken != ""
}

func unauthorized(writer http.ResponseWriter) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusUnauthorized)
	_, _ = writer.Write([]byte(`{"error":"request failed"}`))
}
