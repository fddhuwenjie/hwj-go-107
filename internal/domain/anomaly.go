package domain

// 异常事件状态。
const (
	AnomalyOpen      = "open"
	AnomalyIsolating = "isolating"
	AnomalyDisposing = "disposing"
	AnomalyReviewing = "reviewing"
	AnomalyClosed    = "closed"
)

// 异常严重级别。
const (
	SeverityMinor    = "minor"
	SeverityMajor    = "major"
	SeverityCritical = "critical"
)

// 保护处置状态。
const (
	DisposalPending      = "pending"
	DisposalDone         = "done"
	DisposalReviewPass   = "review_pass"
	DisposalReviewReject = "review_reject"
)

// AnomalyEvent 异常事件。
type AnomalyEvent struct {
	ID            int64  `json:"id"`
	StorageUnitID int64  `json:"storage_unit_id"`
	RuleVersionID int64  `json:"rule_version_id"`
	SampleID      int64  `json:"sample_id"`
	Severity      string `json:"severity"`
	Status        string `json:"status"`
	BreachCount   int    `json:"breach_count"`
	Title         string `json:"title"`
	Version       int64  `json:"version"`
	OpenedAt      int64  `json:"opened_at"`
	ClosedAt      *int64 `json:"closed_at,omitempty"`
}

// IsOpen 是否为未关闭异常。
func (e *AnomalyEvent) IsOpen() bool { return e.Status != AnomalyClosed }

// ProtectionAction 保护处置记录。
type ProtectionAction struct {
	ID         int64  `json:"id"`
	EventID    int64  `json:"event_id"`
	ActionType string `json:"action_type"`
	Operator   string `json:"operator"`
	Note       string `json:"note"`
	Status     string `json:"status"`
	ReviewedBy string `json:"reviewed_by,omitempty"`
	ReviewedAt *int64 `json:"reviewed_at,omitempty"`
	CreatedAt  int64  `json:"created_at"`
}
