package domain

import "encoding/json"

// 借展申请状态。
const (
	LoanDraft      = "draft"
	LoanSubmitted  = "submitted"
	LoanApproved   = "approved"
	LoanInTransit  = "in_transit"
	LoanExhibiting = "exhibiting"
	LoanReturned   = "returned"
	LoanClosed     = "closed"
	LoanRejected   = "rejected"
	LoanCancelled  = "cancelled"
)

// 清点方向。
const (
	CheckOut = "out"
	CheckIn  = "in"
)

// 验收结果。
const (
	AcceptPass          = "pass"
	AcceptPassWithNotes = "pass_with_notes"
	AcceptRejected      = "rejected"
)

// LoanApplication 借展申请。
type LoanApplication struct {
	ID           int64  `json:"id"`
	Code         string `json:"code"`
	Borrower     string `json:"borrower"`
	Venue        string `json:"venue"`
	Purpose      string `json:"purpose"`
	StartAt      int64  `json:"start_at"`
	EndAt        int64  `json:"end_at"`
	Status       string `json:"status"`
	RuleSnapshot string `json:"rule_snapshot,omitempty"`
	ApprovedBy   string `json:"approved_by,omitempty"`
	ApprovedAt   *int64 `json:"approved_at,omitempty"`
	RejectReason string `json:"reject_reason,omitempty"`
	Overdue      bool   `json:"overdue"`
	Attention    bool   `json:"attention"`
	Version      int64  `json:"version"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// PackagingEntry 包装清单条目（冻结快照用）。
type PackagingEntry struct {
	AttachmentID int64  `json:"attachment_id"`
	Name         string `json:"name"`
	Spec         string `json:"spec"`
}

// FrozenRules 审批时冻结的规则快照。
type FrozenRules struct {
	PreLoanWindowSeconds int64                  `json:"pre_loan_window_seconds"`
	Rules                []ThresholdRuleVersion `json:"rules"`
}

// MarshalPackaging 序列化包装清单。
func MarshalPackaging(entries []PackagingEntry) string {
	if entries == nil {
		entries = []PackagingEntry{}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}

// UnmarshalPackaging 反序列化包装清单。
func UnmarshalPackaging(s string) ([]PackagingEntry, error) {
	if s == "" {
		return []PackagingEntry{}, nil
	}
	var out []PackagingEntry
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// LoanItem 借展藏品行，含冻结快照。
type LoanItem struct {
	ID                int64  `json:"id"`
	LoanID            int64  `json:"loan_id"`
	ArtifactID        int64  `json:"artifact_id"`
	FrozenStatus      string `json:"frozen_status"`
	FrozenLevelID     int64  `json:"frozen_level_id"`
	FrozenUnitID      int64  `json:"frozen_unit_id"`
	PackagingSnapshot string `json:"packaging_snapshot"`
	CreatedAt         int64  `json:"created_at"`
}

// InventoryCheck 出入库清点单。
type InventoryCheck struct {
	ID             int64  `json:"id"`
	LoanID         int64  `json:"loan_id"`
	Direction      string `json:"direction"`
	IdempotencyKey string `json:"idempotency_key"`
	Operator       string `json:"operator"`
	Complete       bool   `json:"complete"`
	CheckedAt      int64  `json:"checked_at"`
	CreatedAt      int64  `json:"created_at"`
}

// InventoryCheckItem 清点明细，AttachmentID 为 0 表示藏品本体行。
type InventoryCheckItem struct {
	ID           int64  `json:"id"`
	CheckID      int64  `json:"check_id"`
	ArtifactID   int64  `json:"artifact_id"`
	AttachmentID int64  `json:"attachment_id,omitempty"`
	Present      bool   `json:"present"`
	Note         string `json:"note,omitempty"`
}

// PackageHandover 包装交接记录。
type PackageHandover struct {
	ID             int64  `json:"id"`
	LoanID         int64  `json:"loan_id"`
	Seq            int    `json:"seq"`
	FromPerson     string `json:"from_person"`
	ToPerson       string `json:"to_person"`
	HandedAt       int64  `json:"handed_at"`
	Location       string `json:"location"`
	IdempotencyKey string `json:"idempotency_key"`
	CreatedAt      int64  `json:"created_at"`
}

// TransportNode 运输节点。
type TransportNode struct {
	ID         int64  `json:"id"`
	LoanID     int64  `json:"loan_id"`
	Seq        int    `json:"seq"`
	NodeType   string `json:"node_type"`
	Location   string `json:"location"`
	OccurredAt int64  `json:"occurred_at"`
	RecordedBy string `json:"recorded_by"`
	CreatedAt  int64  `json:"created_at"`
}

// ExhibitionConfirm 展陈确认。
type ExhibitionConfirm struct {
	ID          int64  `json:"id"`
	LoanID      int64  `json:"loan_id"`
	ShowcaseID  int64  `json:"showcase_id"`
	ConfirmedBy string `json:"confirmed_by"`
	ConfirmedAt int64  `json:"confirmed_at"`
	Note        string `json:"note,omitempty"`
	CreatedAt   int64  `json:"created_at"`
}

// ReturnAcceptance 归还验收。
type ReturnAcceptance struct {
	ID         int64  `json:"id"`
	LoanID     int64  `json:"loan_id"`
	CheckID    int64  `json:"check_id"`
	Result     string `json:"result"`
	Reviewer   string `json:"reviewer"`
	Note       string `json:"note,omitempty"`
	ReviewedAt int64  `json:"reviewed_at"`
	CreatedAt  int64  `json:"created_at"`
}
