package domain

// 藏品状态枚举。
const (
	ArtifactRegistered      = "registered"       // 已登记
	ArtifactStored          = "stored"           // 在库
	ArtifactIsolated        = "isolated"         // 已隔离
	ArtifactFrozen          = "frozen"           // 借展冻结
	ArtifactOut             = "out"              // 已出库在途
	ArtifactOnLoan          = "on_loan"          // 展陈中
	ArtifactReturnedPending = "returned_pending" // 归还待验收
	ArtifactRetired         = "retired"          // 已注销
)

// Artifact 藏品。
type Artifact struct {
	ID            int64  `json:"id"`
	Code          string `json:"code"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Era           string `json:"era"`
	Description   string `json:"description"`
	Status        string `json:"status"`
	LevelID       int64  `json:"level_id"`
	StorageUnitID *int64 `json:"storage_unit_id,omitempty"`
	Note          string `json:"note,omitempty"`
	Version       int64  `json:"version"`
	Retired       bool   `json:"retired"`
	RetiredReason string `json:"retired_reason,omitempty"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

// OnActiveLoan 是否处于借展占用状态（冻结/在途/在展/待验收）。
func (a *Artifact) OnActiveLoan() bool {
	switch a.Status {
	case ArtifactFrozen, ArtifactOut, ArtifactOnLoan, ArtifactReturnedPending:
		return true
	}
	return false
}

// CanRetire 是否允许注销。
func (a *Artifact) CanRetire() bool {
	return !a.Retired && (a.Status == ArtifactRegistered || a.Status == ArtifactStored)
}

// Editable 是否允许直接修改基础信息。
func (a *Artifact) Editable() bool {
	return !a.Retired && !a.OnActiveLoan()
}

// Attachment 藏品附件。
type Attachment struct {
	ID         int64  `json:"id"`
	ArtifactID int64  `json:"artifact_id"`
	Name       string `json:"name"`
	Spec       string `json:"spec"`
	CreatedAt  int64  `json:"created_at"`
}

// ArtifactSnapshot 藏品状态历史快照，每次变化追加一条。
type ArtifactSnapshot struct {
	ID            int64  `json:"id"`
	ArtifactID    int64  `json:"artifact_id"`
	Status        string `json:"status"`
	LevelID       int64  `json:"level_id"`
	StorageUnitID *int64 `json:"storage_unit_id,omitempty"`
	Note          string `json:"note,omitempty"`
	Version       int64  `json:"version"`
	Reason        string `json:"reason"`
	CreatedAt     int64  `json:"created_at"`
}
