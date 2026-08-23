package httpx

import (
	"database/sql"
	"net/http"

	"gowork/internal/clock"
)

// HealthHandler 健康检查。
func HealthHandler(db *sql.DB, clk clock.Clock) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := "up"
		code := http.StatusOK
		if err := db.PingContext(r.Context()); err != nil {
			status = "down"
			code = http.StatusServiceUnavailable
		}
		WriteJSON(w, code, map[string]any{
			"status": "ok",
			"time":   clk.Now().Unix(),
			"db":     status,
		})
	}
}
