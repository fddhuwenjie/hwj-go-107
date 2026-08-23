package domain

// 后台作业状态。
const (
	JobPending = "pending"
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// 后台作业类型。
const (
	JobKindPatrol          = "patrol"           // 环境巡检
	JobKindLoanDue         = "loan_due"         // 借展到期
	JobKindHandoverTimeout = "handover_timeout" // 交接超时
)

// AuditLog 审计日志。
type AuditLog struct {
	ID         int64  `json:"id"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   int64  `json:"entity_id"`
	Detail     string `json:"detail"`
	CreatedAt  int64  `json:"created_at"`
}

// Job 后台作业。
type Job struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Payload     string `json:"payload"`
	Status      string `json:"status"`
	Attempts    int    `json:"attempts"`
	MaxAttempts int    `json:"max_attempts"`
	RunAt       int64  `json:"run_at"`
	LastError   string `json:"last_error,omitempty"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}
