package httpx

import (
	"net/http"

	"gowork/internal/repository"
	"gowork/internal/service"
)

// AnomalyHandlers 异常与处置接口。
type AnomalyHandlers struct {
	svc *service.AnomalyService
}

// NewAnomalyHandlers 构造异常接口。
func NewAnomalyHandlers(svc *service.AnomalyService) *AnomalyHandlers {
	return &AnomalyHandlers{svc: svc}
}

// List GET /api/v1/anomalies
func (h *AnomalyHandlers) List(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	unitID, err := QueryInt64(r, "storage_unit_id")
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.List(r.Context(), repository.AnomalyFilter{
		Status: QueryString(r, "status"), StorageUnitID: unitID,
	}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// Get GET /api/v1/anomalies/{id}
func (h *AnomalyHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	e, err := h.svc.Get(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, e)
}

// Isolate POST /api/v1/anomalies/{id}/isolate
func (h *AnomalyHandlers) Isolate(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Operator   string `json:"operator"`
		ActionType string `json:"action_type"`
		Note       string `json:"note"`
		Version    int64  `json:"version"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.Isolate(r.Context(), id, req.Version, req.Operator, req.ActionType, req.Note)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, a)
}

// AddDisposal POST /api/v1/anomalies/{id}/disposals
func (h *AnomalyHandlers) AddDisposal(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Operator   string `json:"operator"`
		ActionType string `json:"action_type"`
		Note       string `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.AddDisposal(r.Context(), id, req.Operator, req.ActionType, req.Note)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, a)
}

// ListDisposals GET /api/v1/anomalies/{id}/disposals
func (h *AnomalyHandlers) ListDisposals(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListDisposals(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

// Review POST /api/v1/disposals/{id}/review
func (h *AnomalyHandlers) Review(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Reviewer string `json:"reviewer"`
		Pass     bool   `json:"pass"`
		Note     string `json:"note"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	e, err := h.svc.Review(r.Context(), id, req.Reviewer, req.Pass, req.Note)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, e)
}
