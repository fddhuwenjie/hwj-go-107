package httpx

import (
	"net/http"

	"gowork/internal/repository"
	"gowork/internal/service"
)

// LoanHandlers 借展全流程接口。
type LoanHandlers struct {
	loans     *service.LoanService
	checks    *service.CheckService
	handovers *service.HandoverService
	returns   *service.ReturnService
	queries   *service.QueryService
}

// NewLoanHandlers 构造借展接口。
func NewLoanHandlers(loans *service.LoanService, checks *service.CheckService,
	handovers *service.HandoverService, returns *service.ReturnService, queries *service.QueryService) *LoanHandlers {
	return &LoanHandlers{loans: loans, checks: checks, handovers: handovers, returns: returns, queries: queries}
}

// Create POST /api/v1/loans
func (h *LoanHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string  `json:"code"`
		Borrower    string  `json:"borrower"`
		Venue       string  `json:"venue"`
		Purpose     string  `json:"purpose"`
		StartAt     int64   `json:"start_at"`
		EndAt       int64   `json:"end_at"`
		ArtifactIDs []int64 `json:"artifact_ids"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	l, err := h.loans.Create(r.Context(), service.CreateLoanInput{
		Code: req.Code, Borrower: req.Borrower, Venue: req.Venue, Purpose: req.Purpose,
		StartAt: req.StartAt, EndAt: req.EndAt, ArtifactIDs: req.ArtifactIDs,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, l)
}

// List GET /api/v1/loans
func (h *LoanHandlers) List(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.loans.List(r.Context(), repository.LoanFilter{Status: QueryString(r, "status")}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// Get GET /api/v1/loans/{id}
func (h *LoanHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	detail, err := h.queries.LoanDetail(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, detail)
}

// versioned 提取 version 字段的通用请求体。
type versioned struct {
	Version int64 `json:"version"`
}

func (h *LoanHandlers) simpleTransition(w http.ResponseWriter, r *http.Request,
	fn func(id, version int64) (any, error)) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req versioned
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	out, err := fn(id, req.Version)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// Submit POST /api/v1/loans/{id}/submit
func (h *LoanHandlers) Submit(w http.ResponseWriter, r *http.Request) {
	h.simpleTransition(w, r, func(id, v int64) (any, error) {
		return h.loans.Submit(r.Context(), id, v)
	})
}

// Cancel POST /api/v1/loans/{id}/cancel
func (h *LoanHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	h.simpleTransition(w, r, func(id, v int64) (any, error) {
		return h.loans.Cancel(r.Context(), id, v)
	})
}

// Reject POST /api/v1/loans/{id}/reject
func (h *LoanHandlers) Reject(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Version  int64  `json:"version"`
		Reviewer string `json:"reviewer"`
		Reason   string `json:"reason"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	l, err := h.loans.Reject(r.Context(), id, req.Version, req.Reviewer, req.Reason)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, l)
}

// Approve POST /api/v1/loans/{id}/approve
func (h *LoanHandlers) Approve(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Version  int64  `json:"version"`
		Reviewer string `json:"reviewer"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	l, err := h.loans.Approve(r.Context(), id, req.Version, req.Reviewer)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, l)
}

// OutCheck POST /api/v1/loans/{id}/out-check
func (h *LoanHandlers) OutCheck(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string                   `json:"idempotency_key"`
		Operator       string                   `json:"operator"`
		Items          []service.CheckItemInput `json:"items"`
		Handover       service.HandoverInput    `json:"handover"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	res, err := h.checks.OutCheck(r.Context(), id, req.IdempotencyKey, req.Operator, req.Items, req.Handover)
	if err != nil {
		WriteError(w, err)
		return
	}
	status := http.StatusOK
	if !res.IdempotentReplay {
		status = http.StatusCreated
	}
	WriteJSON(w, status, res)
}

// InCheck POST /api/v1/loans/{id}/in-check
func (h *LoanHandlers) InCheck(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string                   `json:"idempotency_key"`
		Operator       string                   `json:"operator"`
		Items          []service.CheckItemInput `json:"items"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	res, err := h.checks.InCheck(r.Context(), id, req.IdempotencyKey, req.Operator, req.Items)
	if err != nil {
		WriteError(w, err)
		return
	}
	status := http.StatusOK
	if !res.IdempotentReplay {
		status = http.StatusCreated
	}
	WriteJSON(w, status, res)
}

// AddHandover POST /api/v1/loans/{id}/handovers
func (h *LoanHandlers) AddHandover(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		IdempotencyKey string `json:"idempotency_key"`
		FromPerson     string `json:"from_person"`
		ToPerson       string `json:"to_person"`
		HandedAt       int64  `json:"handed_at"`
		Location       string `json:"location"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	ho, replay, err := h.handovers.AddHandover(r.Context(), id, req.IdempotencyKey, service.HandoverInput{
		FromPerson: req.FromPerson, ToPerson: req.ToPerson, HandedAt: req.HandedAt, Location: req.Location,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	WriteJSON(w, status, map[string]any{"handover": ho, "idempotent_replay": replay})
}

// AddTransportNode POST /api/v1/loans/{id}/transport-nodes
func (h *LoanHandlers) AddTransportNode(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		NodeType   string `json:"node_type"`
		Location   string `json:"location"`
		OccurredAt int64  `json:"occurred_at"`
		RecordedBy string `json:"recorded_by"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	n, err := h.handovers.AddTransportNode(r.Context(), id, req.NodeType, req.Location, req.OccurredAt, req.RecordedBy)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, n)
}

// ConfirmExhibition POST /api/v1/loans/{id}/exhibition-confirm
func (h *LoanHandlers) ConfirmExhibition(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		ShowcaseID  int64  `json:"showcase_id"`
		ConfirmedBy string `json:"confirmed_by"`
		Note        string `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	c, err := h.handovers.ConfirmExhibition(r.Context(), id, req.ShowcaseID, req.ConfirmedBy, req.Note)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, c)
}

// ReturnAcceptance POST /api/v1/loans/{id}/return-acceptance
func (h *LoanHandlers) ReturnAcceptance(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Result   string `json:"result"`
		Reviewer string `json:"reviewer"`
		Note     string `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.returns.Accept(r.Context(), id, req.Result, req.Reviewer, req.Note)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, a)
}
