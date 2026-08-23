package domain

// 阈值规则版本状态。
const (
	RuleDraft   = "draft"
	RuleActive  = "active"
	RuleRetired = "retired"
)

// PreservationLevel 保存等级。
type PreservationLevel struct {
	ID          int64  `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"created_at"`
}

// ThresholdRuleVersion 阈值规则版本。
type ThresholdRuleVersion struct {
	ID                int64   `json:"id"`
	LevelID           int64   `json:"level_id"`
	VersionNo         int     `json:"version_no"`
	TempMin           float64 `json:"temp_min"`
	TempMax           float64 `json:"temp_max"`
	HumidityMin       float64 `json:"humidity_min"`
	HumidityMax       float64 `json:"humidity_max"`
	ConsecutiveBreach int     `json:"consecutive_breach"`
	Status            string  `json:"status"`
	CreatedAt         int64   `json:"created_at"`
	ActivatedAt       *int64  `json:"activated_at,omitempty"`
}

// EnvSample 环境采样，只增不改，即环境历史快照。
type EnvSample struct {
	ID            int64   `json:"id"`
	SensorID      int64   `json:"sensor_id"`
	StorageUnitID int64   `json:"storage_unit_id"`
	Temperature   float64 `json:"temperature"`
	Humidity      float64 `json:"humidity"`
	SampledAt     int64   `json:"sampled_at"`
	ReceivedAt    int64   `json:"received_at"`
	Late          bool    `json:"late"`
	Processed     bool    `json:"processed"`
}
