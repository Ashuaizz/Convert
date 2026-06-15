package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"convert-backend/internal/pkg/idgen"
)

type contextKey string

const RequestIDKey contextKey = "request_id"

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = idgen.New("req")
		}
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(RequestIDKey).(string)
	return requestID
}

func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"request_id": RequestIDFromContext(r.Context()),
					"code":       "INTERNAL_ERROR",
					"message":    "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
