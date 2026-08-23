package httpx

import (
	"net/http"

	"gowork/internal/repository"
	"gowork/internal/service"
)

// EnvHandlers 环境域接口：单元、传感器、等级、规则、采样。
type EnvHandlers struct {
	svc *service.EnvService
}

// NewEnvHandlers 构造环境域接口。
func NewEnvHandlers(svc *service.EnvService) *EnvHandlers { return &EnvHandlers{svc: svc} }

// CreateUnit POST /api/v1/storage-units
func (h *EnvHandlers) CreateUnit(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Kind     string `json:"kind"`
		Location string `json:"location"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	u, err := h.svc.CreateUnit(r.Context(), req.Code, req.Name, req.Kind, req.Location)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, u)
}

// ListUnits GET /api/v1/storage-units
func (h *EnvHandlers) ListUnits(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListUnits(r.Context(), repository.UnitFilter{Kind: QueryString(r, "kind")}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateSensor POST /api/v1/sensors
func (h *EnvHandlers) CreateSensor(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code          string `json:"code"`
		StorageUnitID int64  `json:"storage_unit_id"`
		Kind          string `json:"kind"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	s, err := h.svc.CreateSensor(r.Context(), req.Code, req.StorageUnitID, req.Kind)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, s)
}

// ListSensors GET /api/v1/sensors
func (h *EnvHandlers) ListSensors(w http.ResponseWriter, r *http.Request) {
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
	out, err := h.svc.ListSensors(r.Context(), repository.SensorFilter{StorageUnitID: unitID}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateLevel POST /api/v1/preservation-levels
func (h *EnvHandlers) CreateLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	l, err := h.svc.CreateLevel(r.Context(), req.Code, req.Name, req.Description)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, l)
}

// ListLevels GET /api/v1/preservation-levels
func (h *EnvHandlers) ListLevels(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListLevels(r.Context(), p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// CreateRule POST /api/v1/threshold-rules
func (h *EnvHandlers) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LevelID           int64   `json:"level_id"`
		TempMin           float64 `json:"temp_min"`
		TempMax           float64 `json:"temp_max"`
		HumidityMin       float64 `json:"humidity_min"`
		HumidityMax       float64 `json:"humidity_max"`
		ConsecutiveBreach int     `json:"consecutive_breach"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	rv, err := h.svc.CreateRule(r.Context(), req.LevelID, req.TempMin, req.TempMax,
		req.HumidityMin, req.HumidityMax, req.ConsecutiveBreach)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, rv)
}

// ActivateRule POST /api/v1/threshold-rules/{id}/activate
func (h *EnvHandlers) ActivateRule(w http.ResponseWriter, r *http.Request) {
	id, err := ParseID(PathValue(r, "id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	rv, err := h.svc.ActivateRule(r.Context(), id)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, rv)
}

// ListRules GET /api/v1/threshold-rules
func (h *EnvHandlers) ListRules(w http.ResponseWriter, r *http.Request) {
	p, err := ParsePage(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	levelID, err := QueryInt64(r, "level_id")
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListRules(r.Context(), repository.RuleFilter{
		LevelID: levelID, Status: QueryString(r, "status"),
	}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// IngestSample POST /api/v1/env-samples
func (h *EnvHandlers) IngestSample(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SensorID    int64   `json:"sensor_id"`
		Temperature float64 `json:"temperature"`
		Humidity    float64 `json:"humidity"`
		SampledAt   int64   `json:"sampled_at"`
	}
	if err := DecodeJSON(r, &req); err != nil {
		WriteError(w, err)
		return
	}
	s, err := h.svc.IngestSample(r.Context(), req.SensorID, req.Temperature, req.Humidity, req.SampledAt)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, s)
}

// ListSamples GET /api/v1/env-samples
func (h *EnvHandlers) ListSamples(w http.ResponseWriter, r *http.Request) {
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
	sensorID, err := QueryInt64(r, "sensor_id")
	if err != nil {
		WriteError(w, err)
		return
	}
	out, err := h.svc.ListSamples(r.Context(), repository.SampleFilter{
		StorageUnitID: unitID, SensorID: sensorID,
	}, p)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}
