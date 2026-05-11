// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// GitHub: https://github.com/ArnaudGuiovanna
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const LearnerIDKey contextKey = "learner_id"

func BearerMiddleware(baseURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		tokenStr, ok := bearerToken(authHeader)
		if !ok {
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer resource_metadata="%s/.well-known/oauth-protected-resource"`, baseURL,
			))
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		learnerID, err := VerifyJWT(tokenStr, baseURL)
		if err != nil {
			slog.Debug("jwt verify failed", "err", err)
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(
				`Bearer resource_metadata="%s/.well-known/oauth-protected-resource", error="invalid_token"`, baseURL,
			))
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), LearnerIDKey, learnerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(authHeader string) (string, bool) {
	scheme, token, ok := strings.Cut(authHeader, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}
	return token, true
}

func GetLearnerID(ctx context.Context) string {
	id, _ := ctx.Value(LearnerIDKey).(string)
	return id
}
