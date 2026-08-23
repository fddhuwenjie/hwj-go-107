package repository

import (
	"context"

	"gowork/internal/domain"
)

// LoanFilter 借展过滤。
type LoanFilter struct {
	Status *string
}

// LoanRepository 借展申请与借展藏品仓储。
type LoanRepository interface {
	Create(ctx context.Context, l *domain.LoanApplication) error
	GetByID(ctx context.Context, id int64) (*domain.LoanApplication, error)
	// Update 乐观锁更新。
	Update(ctx context.Context, l *domain.LoanApplication) error
	List(ctx context.Context, f LoanFilter, p domain.Page) ([]domain.LoanApplication, error)
	ListByStatus(ctx context.Context, statuses ...string) ([]domain.LoanApplication, error)
	AddItem(ctx context.Context, it *domain.LoanItem) error
	ItemsByLoan(ctx context.Context, loanID int64) ([]domain.LoanItem, error)
	// FreezeItem 审批时回写冻结快照。
	FreezeItem(ctx context.Context, it *domain.LoanItem) error
	// LatestAcceptanceTimeByUnit 查询某单元相关借展最近一次验收完成时间（用于迟到数据判定）。
	LatestAcceptanceTimeByUnit(ctx context.Context, unitID int64) (int64, bool, error)
}

// CheckRepository 出入库清点仓储。
type CheckRepository interface {
	Create(ctx context.Context, c *domain.InventoryCheck) error
	CreateItem(ctx context.Context, it *domain.InventoryCheckItem) error
	ItemsByCheck(ctx context.Context, checkID int64) ([]domain.InventoryCheckItem, error)
	ByLoanAndDirection(ctx context.Context, loanID int64, direction string) (*domain.InventoryCheck, error)
	ByIdempotencyKey(ctx context.Context, key string) (*domain.InventoryCheck, error)
}

// HandoverRepository 包装交接与运输节点仓储。
type HandoverRepository interface {
	Create(ctx context.Context, h *domain.PackageHandover) error
	ByIdempotencyKey(ctx context.Context, key string) (*domain.PackageHandover, error)
	LatestByLoan(ctx context.Context, loanID int64) (*domain.PackageHandover, error)
	ListByLoan(ctx context.Context, loanID int64) ([]domain.PackageHandover, error)
	CreateNode(ctx context.Context, n *domain.TransportNode) error
	LatestNodeByLoan(ctx context.Context, loanID int64) (*domain.TransportNode, error)
	ListNodesByLoan(ctx context.Context, loanID int64) ([]domain.TransportNode, error)
}

// AcceptanceRepository 展陈确认与归还验收仓储。
type AcceptanceRepository interface {
	CreateConfirm(ctx context.Context, c *domain.ExhibitionConfirm) error
	ConfirmByLoan(ctx context.Context, loanID int64) (*domain.ExhibitionConfirm, error)
	CreateAcceptance(ctx context.Context, a *domain.ReturnAcceptance) error
	AcceptanceByLoan(ctx context.Context, loanID int64) (*domain.ReturnAcceptance, error)
}
