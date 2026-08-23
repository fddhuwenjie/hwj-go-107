package httpx

import (
	"net/http"
	"strconv"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/service"
)

// QueryHandlers 专题查询接口。
type QueryHandlers struct {
	svc *service.QueryService
}

// NewQueryHandlers 构造查询接口。
func NewQueryHandlers(svc *service.QueryService) *QueryHandlers { return &QueryHandlers{svc: svc} }

// UpcomingWithAnomalies GET /api/v1/queries/loans/upcoming-with-anomalies
func (h *QueryHandlers) UpcomingWithAnomalies(w http.ResponseWriter, r *http.Request) {
	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		var n int
		if _, err := parseInt(d, &n); err != nil || n <= 0 {
			WriteError(w, badDays(d))
			return
		}
		days = n
	}
	out, err := h.svc.UpcomingWithAnomalies(r.Context(), days)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func parseInt(s string, out *int) (int, error) {
	n, err := strconv.Atoi(s)
	*out = n
	return n, err
}

func badDays(v string) error { return domain.Invalidf("非法天数 %q", v) }

// WarehouseRiskRanking GET /api/v1/queries/warehouses/risk-ranking
func (h *QueryHandlers) WarehouseRiskRanking(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.WarehouseRiskRanking(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ConsecutiveBreaches GET /api/v1/queries/sensors/consecutive-breaches
func (h *QueryHandlers) ConsecutiveBreaches(w http.ResponseWriter, r *http.Request) {
	min := 3
	if m := r.URL.Query().Get("min"); m != "" {
		var n int
		if _, err := parseInt(m, &n); err != nil || n <= 0 {
			WriteError(w, domain.Invalidf("非法最小连续次数 %q", m))
			return
		}
		min = n
	}
	out, err := h.svc.ConsecutiveBreachSensors(r.Context(), min)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// PackagingDiff GET /api/v1/queries/loans/{id}/packaging-diff
func (h *QueryHandlers) PackagingDiff(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	direction := r.URL.Query().Get("direction")
	if direction == "" {
		direction = "in"
	}
	out, err := h.svc.PackagingDiff(r.Context(), id, direction)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// OverdueLoans GET /api/v1/queries/loans/overdue
func (h *QueryHandlers) OverdueLoans(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.OverdueLoans(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// AuditHandlers 审计查询接口。
type AuditHandlers struct {
	repo *repository.Repositories
}

// NewAuditHandlers 构造审计接口。
func NewAuditHandlers(repo *repository.Repositories) *AuditHandlers {
	return &AuditHandlers{repo: repo}
}

// List GET /api/v1/audit-logs
func (h *AuditHandlers) List(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	entityID, err := QueryInt64(r, "entity_id")
	if err != nil {
		WriteError(w, err)
		return
	}
	items, err := h.repo.Audit.List(r.Context(), repository.AuditFilter{
		EntityType: QueryString(r, "entity_type"), EntityID: entityID,
	}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	out := domain.BuildPaged(items, p.Limit, func(l domain.AuditLog) int64 { return l.ID })
	WriteJSON(w, http.StatusOK, out)
}
