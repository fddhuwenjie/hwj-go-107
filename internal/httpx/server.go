package httpx

import (
	"database/sql"
	"log/slog"
	"net/http"

	"gowork/internal/clock"
	"gowork/internal/repository"
	"gowork/internal/service"
)

// Server 依赖聚合。
type Server struct {
	Artifacts *service.ArtifactService
	Env       *service.EnvService
	Anomalies *service.AnomalyService
	Loans     *service.LoanService
	Checks    *service.CheckService
	Handovers *service.HandoverService
	Returns   *service.ReturnService
	Queries   *service.QueryService
	Repo      *repository.Repositories
	DB        *sql.DB
	Clock     clock.Clock
	Log       *slog.Logger
}

// NewHandler 组装全部路由。
func NewHandler(s *Server) http.Handler {
	rt := NewRouter()

	rt.Handle("GET /healthz", HealthHandler(s.DB, s.Clock))

	ah := NewArtifactHandlers(s.Artifacts)
	rt.Handle("POST /api/v1/artifacts", ah.Register)
	rt.Handle("GET /api/v1/artifacts", ah.List)
	rt.Handle("GET /api/v1/artifacts/{id}", ah.Get)
	rt.Handle("PATCH /api/v1/artifacts/{id}", ah.Update)
	rt.Handle("POST /api/v1/artifacts/{id}/assign-location", ah.AssignLocation)
	rt.Handle("POST /api/v1/artifacts/{id}/retire", ah.Retire)
	rt.Handle("GET /api/v1/artifacts/{id}/snapshots", ah.Snapshots)
	rt.Handle("POST /api/v1/artifacts/{id}/attachments", ah.AddAttachment)
	rt.Handle("GET /api/v1/artifacts/{id}/attachments", ah.ListAttachments)

	eh := NewEnvHandlers(s.Env)
	rt.Handle("POST /api/v1/storage-units", eh.CreateUnit)
	rt.Handle("GET /api/v1/storage-units", eh.ListUnits)
	rt.Handle("POST /api/v1/sensors", eh.CreateSensor)
	rt.Handle("GET /api/v1/sensors", eh.ListSensors)
	rt.Handle("POST /api/v1/preservation-levels", eh.CreateLevel)
	rt.Handle("GET /api/v1/preservation-levels", eh.ListLevels)
	rt.Handle("POST /api/v1/threshold-rules", eh.CreateRule)
	rt.Handle("GET /api/v1/threshold-rules", eh.ListRules)
	rt.Handle("POST /api/v1/threshold-rules/{id}/activate", eh.ActivateRule)
	rt.Handle("POST /api/v1/env-samples", eh.IngestSample)
	rt.Handle("GET /api/v1/env-samples", eh.ListSamples)

	nh := NewAnomalyHandlers(s.Anomalies)
	rt.Handle("GET /api/v1/anomalies", nh.List)
	rt.Handle("GET /api/v1/anomalies/{id}", nh.Get)
	rt.Handle("POST /api/v1/anomalies/{id}/isolate", nh.Isolate)
	rt.Handle("POST /api/v1/anomalies/{id}/disposals", nh.AddDisposal)
	rt.Handle("GET /api/v1/anomalies/{id}/disposals", nh.ListDisposals)
	rt.Handle("POST /api/v1/disposals/{id}/review", nh.Review)

	lh := NewLoanHandlers(s.Loans, s.Checks, s.Handovers, s.Returns, s.Queries)
	rt.Handle("POST /api/v1/loans", lh.Create)
	rt.Handle("GET /api/v1/loans", lh.List)
	rt.Handle("GET /api/v1/loans/{id}", lh.Get)
	rt.Handle("POST /api/v1/loans/{id}/submit", lh.Submit)
	rt.Handle("POST /api/v1/loans/{id}/cancel", lh.Cancel)
	rt.Handle("POST /api/v1/loans/{id}/reject", lh.Reject)
	rt.Handle("POST /api/v1/loans/{id}/approve", lh.Approve)
	rt.Handle("POST /api/v1/loans/{id}/out-check", lh.OutCheck)
	rt.Handle("POST /api/v1/loans/{id}/in-check", lh.InCheck)
	rt.Handle("POST /api/v1/loans/{id}/handovers", lh.AddHandover)
	rt.Handle("POST /api/v1/loans/{id}/transport-nodes", lh.AddTransportNode)
	rt.Handle("POST /api/v1/loans/{id}/exhibition-confirm", lh.ConfirmExhibition)
	rt.Handle("POST /api/v1/loans/{id}/return-acceptance", lh.ReturnAcceptance)

	qh := NewQueryHandlers(s.Queries)
	rt.Handle("GET /api/v1/queries/loans/upcoming-with-anomalies", qh.UpcomingWithAnomalies)
	rt.Handle("GET /api/v1/queries/loans/overdue", qh.OverdueLoans)
	rt.Handle("GET /api/v1/queries/warehouses/risk-ranking", qh.WarehouseRiskRanking)
	rt.Handle("GET /api/v1/queries/sensors/consecutive-breaches", qh.ConsecutiveBreaches)
	rt.Handle("GET /api/v1/queries/loans/{id}/packaging-diff", qh.PackagingDiff)

	aud := NewAuditHandlers(s.Repo)
	rt.Handle("GET /api/v1/audit-logs", aud.List)

	return RecoverMiddleware(s.Log, LoggingMiddleware(s.Log, rt))
}
