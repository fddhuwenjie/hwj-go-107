package service

import (
	"context"
	"errors"

	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/rules"
)

// QueryService 专题查询服务。
type QueryService struct {
	d *Deps
}

// NewQueryService 构造查询服务。
func NewQueryService(d *Deps) *QueryService { return &QueryService{d: d} }

// UpcomingWithAnomalies 临近借展（days 天内开始）仍有环境异常的藏品。
func (s *QueryService) UpcomingWithAnomalies(ctx context.Context, days int) ([]repository.UpcomingAnomalyRow, error) {
	if days <= 0 {
		days = 7
	}
	now := s.d.now()
	return s.d.Repo.Queries.UpcomingLoansWithAnomalies(ctx, now, now+int64(days)*86400)
}

// WarehouseRiskRanking 库房风险排序（近 24 小时越界采样计入）。
func (s *QueryService) WarehouseRiskRanking(ctx context.Context) ([]repository.WarehouseRiskRow, error) {
	return s.d.Repo.Queries.WarehouseRiskRanking(ctx, s.d.now()-86400)
}

// ConsecutiveBreachRow 连续越界传感器行。
type ConsecutiveBreachRow struct {
	Sensor      domain.Sensor `json:"sensor"`
	UnitID      int64         `json:"storage_unit_id"`
	Consecutive int           `json:"consecutive"`
}

// ConsecutiveBreachSensors 最近连续 min 次采样全部越界的传感器。
func (s *QueryService) ConsecutiveBreachSensors(ctx context.Context, min int) ([]ConsecutiveBreachRow, error) {
	if min <= 0 {
		min = 3
	}
	out := []ConsecutiveBreachRow{}
	cursor := int64(0)
	for {
		page, err := s.d.Repo.Sensors.List(ctx, repository.SensorFilter{}, domain.Page{Cursor: cursor, Limit: 200})
		if err != nil {
			return nil, err
		}
		if len(page) == 0 {
			break
		}
		for _, sensor := range page {
			row, ok, err := s.checkSensor(ctx, sensor, min)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, row)
			}
			cursor = sensor.ID
		}
		if len(page) < 200 {
			break
		}
	}
	return out, nil
}

// checkSensor 判定单个传感器是否连续越界。
func (s *QueryService) checkSensor(ctx context.Context, sensor domain.Sensor, min int) (ConsecutiveBreachRow, bool, error) {
	arts, err := s.d.Repo.Artifacts.ListByUnit(ctx, sensor.StorageUnitID)
	if err != nil {
		return ConsecutiveBreachRow{}, false, err
	}
	if len(arts) == 0 {
		return ConsecutiveBreachRow{}, false, nil
	}
	levelIDs := make([]int64, 0, len(arts))
	for _, a := range arts {
		levelIDs = append(levelIDs, a.LevelID)
	}
	rule, err := s.d.strictestRuleForLevels(ctx, s.d.Repo, unique(levelIDs))
	if err != nil {
		// 无启用规则的单元不参与判定
		return ConsecutiveBreachRow{}, false, nil
	}
	samples, err := s.d.Repo.Samples.ListRecentBySensor(ctx, sensor.ID, 50)
	if err != nil {
		return ConsecutiveBreachRow{}, false, err
	}
	n := rules.ConsecutiveBreaches(samples, rule)
	if n >= min {
		return ConsecutiveBreachRow{Sensor: sensor, UnitID: sensor.StorageUnitID, Consecutive: n}, true, nil
	}
	return ConsecutiveBreachRow{}, false, nil
}

// PackagingDiffRow 包装清单差异行。
type PackagingDiffRow struct {
	ArtifactID    int64  `json:"artifact_id"`
	AttachmentID  int64  `json:"attachment_id,omitempty"`
	Name          string `json:"name"`
	ExpectedCount bool   `json:"expected"`
	Present       bool   `json:"present"`
	Diff          string `json:"diff"` // missing_artifact / missing_attachment / ok
}

// PackagingDiff 冻结包装清单与指定方向清点结果比对。
func (s *QueryService) PackagingDiff(ctx context.Context, loanID int64, direction string) ([]PackagingDiffRow, error) {
	if direction != domain.CheckOut && direction != domain.CheckIn {
		return nil, domain.Invalidf("direction 必须为 out 或 in")
	}
	items, err := s.d.Repo.Loans.ItemsByLoan(ctx, loanID)
	if err != nil {
		return nil, err
	}
	check, err := s.d.Repo.Checks.ByLoanAndDirection(ctx, loanID, direction)
	if err != nil {
		return nil, err
	}
	checkItems, err := s.d.Repo.Checks.ItemsByCheck(ctx, check.ID)
	if err != nil {
		return nil, err
	}
	presentMap := map[[2]int64]bool{}
	for _, ci := range checkItems {
		presentMap[[2]int64{ci.ArtifactID, ci.AttachmentID}] = ci.Present
	}
	out := []PackagingDiffRow{}
	for _, li := range items {
		present := presentMap[[2]int64{li.ArtifactID, 0}]
		row := PackagingDiffRow{ArtifactID: li.ArtifactID, ExpectedCount: true, Present: present, Diff: "ok"}
		if !present {
			row.Diff = "missing_artifact"
		}
		out = append(out, row)
		entries, err := domain.UnmarshalPackaging(li.PackagingSnapshot)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			p := presentMap[[2]int64{li.ArtifactID, e.AttachmentID}]
			ar := PackagingDiffRow{ArtifactID: li.ArtifactID, AttachmentID: e.AttachmentID, Name: e.Name, ExpectedCount: true, Present: p, Diff: "ok"}
			if !p {
				ar.Diff = "missing_attachment"
			}
			out = append(out, ar)
		}
	}
	return out, nil
}

// OverdueLoans 逾期未归还借展（end_at 早于当前且未关闭）。
func (s *QueryService) OverdueLoans(ctx context.Context) ([]domain.LoanApplication, error) {
	loans, err := s.d.Repo.Loans.ListByStatus(ctx, domain.LoanInTransit, domain.LoanExhibiting, domain.LoanReturned)
	if err != nil {
		return nil, err
	}
	now := s.d.now()
	out := []domain.LoanApplication{}
	for _, l := range loans {
		if l.EndAt < now {
			out = append(out, l)
		}
	}
	return out, nil
}

// LoanDetail 借展完整详情。
type LoanDetail struct {
	Loan       domain.LoanApplication    `json:"loan"`
	Items      []domain.LoanItem         `json:"items"`
	Checks     []domain.InventoryCheck   `json:"checks"`
	Handovers  []domain.PackageHandover  `json:"handovers"`
	Nodes      []domain.TransportNode    `json:"transport_nodes"`
	Confirm    *domain.ExhibitionConfirm `json:"exhibition_confirm,omitempty"`
	Acceptance *domain.ReturnAcceptance  `json:"return_acceptance,omitempty"`
}

// LoanDetail 组装借展详情。
func (s *QueryService) LoanDetail(ctx context.Context, loanID int64) (*LoanDetail, error) {
	l, err := s.d.Repo.Loans.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	detail := &LoanDetail{Loan: *l}
	if detail.Items, err = s.d.Repo.Loans.ItemsByLoan(ctx, loanID); err != nil {
		return nil, err
	}
	detail.Checks = []domain.InventoryCheck{}
	directions := []string{domain.CheckOut, domain.CheckIn}
	for _, dir := range directions {
		c, err := s.d.Repo.Checks.ByLoanAndDirection(ctx, loanID, dir)
		if err == nil {
			if c.Direction != dir {
				c.Direction = dir
			}
			detail.Checks = append(detail.Checks, *c)
		} else if !errors.Is(err, domain.ErrNotFound) {
			return nil, err
		}
	}
	if detail.Handovers, err = s.d.Repo.Handovers.ListByLoan(ctx, loanID); err != nil {
		return nil, err
	}
	if detail.Nodes, err = s.d.Repo.Handovers.ListNodesByLoan(ctx, loanID); err != nil {
		return nil, err
	}
	if c, err := s.d.Repo.Acceptances.ConfirmByLoan(ctx, loanID); err == nil {
		detail.Confirm = c
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	if a, err := s.d.Repo.Acceptances.AcceptanceByLoan(ctx, loanID); err == nil {
		detail.Acceptance = a
	} else if !errors.Is(err, domain.ErrNotFound) {
		return nil, err
	}
	return detail, nil
}
