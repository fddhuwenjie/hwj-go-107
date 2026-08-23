package httpx

import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder 记录响应状态码。
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware 结构化访问日志。
func LoggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

// RecoverMiddleware 兜底 panic，返回统一错误。
func RecoverMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if p := recover(); p != nil {
				log.Error("panic", "path", r.URL.Path, "error", p)
				WriteJSON(w, http.StatusInternalServerError, map[string]any{
					"error": map[string]string{"code": "internal", "message": "内部错误"},
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
