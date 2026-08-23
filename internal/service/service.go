// Package service 业务服务层，编排领域规则、仓储与事务。
package service

import (
	"context"

	"gowork/internal/audit"
	"gowork/internal/clock"
	"gowork/internal/config"
	"gowork/internal/domain"
	"gowork/internal/repository"
	"gowork/internal/rules"
	"gowork/internal/tx"
)

// Deps 服务层公共依赖。
type Deps struct {
	// Repo 基于 *sql.DB 的只读/简单写仓储。
	Repo *repository.Repositories
	// Tx 事务管理器，多步写操作必须在事务内执行。
	Tx *tx.Manager
	// Clock 可注入时钟。
	Clock clock.Clock
	// Audit 审计记录器。
	Audit *audit.Recorder
	// Cfg 配置。
	Cfg config.Config
}

// now 当前 Unix 秒。
func (d *Deps) now() int64 { return d.Clock.Now().Unix() }

// strictestRuleForLevels 取多个保存等级启用规则的最严格合并规则。
func (d *Deps) strictestRuleForLevels(ctx context.Context, r *repository.Repositories, levelIDs []int64) (domain.ThresholdRuleVersion, error) {
	active, err := r.Rules.ActiveByLevels(ctx, levelIDs)
	if err != nil {
		return domain.ThresholdRuleVersion{}, err
	}
	// 可能存在部分等级无启用规则：以已启用规则的并集为准
	merged, ok := rules.StrictestRule(active)
	if !ok {
		return domain.ThresholdRuleVersion{}, domain.Rulef("保存等级 %v 缺少启用的阈值规则", levelIDs)
	}
	return merged, nil
}

// checkEnvEligibility 校验单元在 [now-window, now] 前置窗口内环境连续合格且无未关闭异常。
func (d *Deps) checkEnvEligibility(ctx context.Context, r *repository.Repositories, unitID int64, levelIDs []int64, what string) error {
	open, err := r.Anomalies.OpenByUnit(ctx, unitID)
	if err != nil {
		return err
	}
	if len(open) > 0 {
		return domain.Rulef("%s 存在 %d 个未关闭环境异常", what, len(open))
	}
	rule, err := d.strictestRuleForLevels(ctx, r, levelIDs)
	if err != nil {
		return err
	}
	now := d.now()
	from := now - int64(d.Cfg.PreLoanWindow.Seconds())
	samples, err := r.Samples.ListByUnitWindow(ctx, unitID, from, now)
	if err != nil {
		return err
	}
	res := rules.ContinuousQualified(samples, rule, from, now, int64(d.Cfg.EnvMaxGap.Seconds()))
	if !res.Qualified {
		return domain.Rulef("%s 前置环境窗口不合格：%s", what, res.Reason)
	}
	return nil
}

// unique 去重保持顺序。
func unique(ids []int64) []int64 {
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
