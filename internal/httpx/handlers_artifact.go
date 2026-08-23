package httpx

import (
	"net/http"

	"gowork/internal/repository"
	"gowork/internal/service"
)

// ArtifactHandlers 藏品相关接口。
type ArtifactHandlers struct {
	svc *service.ArtifactService
}

// NewArtifactHandlers 构造藏品接口。
func NewArtifactHandlers(svc *service.ArtifactService) *ArtifactHandlers {
	return &ArtifactHandlers{svc: svc}
}

// Register POST /api/v1/artifacts
func (h *ArtifactHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Era         string `json:"era"`
		Description string `json:"description"`
		LevelID     int64  `json:"level_id"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.Register(r.Context(), service.RegisterInput{
		Code: req.Code, Name: req.Name, Category: req.Category,
		Era: req.Era, Description: req.Description, LevelID: req.LevelID,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, a)
}

// List GET /api/v1/artifacts
func (h *ArtifactHandlers) List(w http.ResponseWriter, r *http.Request) {
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
	f := repository.ArtifactFilter{Status: QueryString(r, "status"), StorageUnitID: unitID}
	out, err := h.svc.List(r.Context(), f, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// Get GET /api/v1/artifacts/{id}
func (h *ArtifactHandlers) Get(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.Get(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, a)
}

// Update PATCH /api/v1/artifacts/{id}
func (h *ArtifactHandlers) Update(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Name        string `json:"name"`
		Category    string `json:"category"`
		Era         string `json:"era"`
		Description string `json:"description"`
		Note        string `json:"note"`
		Version     int64  `json:"version"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.Update(r.Context(), id, service.UpdateInput{
		Name: req.Name, Category: req.Category, Era: req.Era,
		Description: req.Description, Note: req.Note, Version: req.Version,
	})
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, a)
}

// AssignLocation POST /api/v1/artifacts/{id}/assign-location
func (h *ArtifactHandlers) AssignLocation(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		StorageUnitID int64 `json:"storage_unit_id"`
		Version       int64 `json:"version"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.AssignLocation(r.Context(), id, req.StorageUnitID, req.Version)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, a)
}

// Retire POST /api/v1/artifacts/{id}/retire
func (h *ArtifactHandlers) Retire(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Reason  string `json:"reason"`
		Version int64  `json:"version"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	a, err := h.svc.Retire(r.Context(), id, req.Version, req.Reason)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, a)
}

// Snapshots GET /api/v1/artifacts/{id}/snapshots
func (h *ArtifactHandlers) Snapshots(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.Snapshots(r.Context(), id, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// AddAttachment POST /api/v1/artifacts/{id}/attachments
func (h *ArtifactHandlers) AddAttachment(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	var req struct {
		Name string `json:"name"`
		Spec string `json:"spec"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	at, err := h.svc.AddAttachment(r.Context(), id, req.Name, req.Spec)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, at)
}

// ListAttachments GET /api/v1/artifacts/{id}/attachments
func (h *ArtifactHandlers) ListAttachments(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListAttachments(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}
