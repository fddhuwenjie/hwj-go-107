package rules

import (
	"gowork/internal/domain"
)

// StrictestRule 从一组启用规则中合并出最严格（各区间取交集）的规则。
// 单元内有多件不同保存等级的藏品时，环境判定采用最严格阈值。
// 输入为空时返回 false。
func StrictestRule(rs []domain.ThresholdRuleVersion) (domain.ThresholdRuleVersion, bool) {
	if len(rs) == 0 {
		return domain.ThresholdRuleVersion{}, false
	}
	out := rs[0]
	for _, r := range rs[1:] {
		if r.TempMin > out.TempMin {
			out.TempMin = r.TempMin
		}
		if r.TempMax < out.TempMax {
			out.TempMax = r.TempMax
		}
		if r.HumidityMin > out.HumidityMin {
			out.HumidityMin = r.HumidityMin
		}
		if r.HumidityMax < out.HumidityMax {
			out.HumidityMax = r.HumidityMax
		}
		if r.ConsecutiveBreach < out.ConsecutiveBreach {
			out.ConsecutiveBreach = r.ConsecutiveBreach
		}
	}
	return out, true
}

// HandoverOrderValid 校验追加交接的有序性：
//   - 交接时间必须严格晚于上一段；
//   - 交出方必须等于上一段接收方（交接链连续）；
//   - 接收方不得与上一段接收方身份重复（防止同一身份连续替代）。
func HandoverOrderValid(prev *domain.PackageHandover, fromPerson, toPerson string, handedAt int64) error {
	if prev == nil {
		if fromPerson == "" || toPerson == "" {
			return domain.Invalidf("交接双方不能为空")
		}
		if fromPerson == toPerson {
			return domain.Rulef("交接双方不能为同一人")
		}
		return nil
	}
	if handedAt <= prev.HandedAt {
		return domain.Rulef("交接时间必须严格递增：上一段为 %d", prev.HandedAt)
	}
	if fromPerson != prev.ToPerson {
		return domain.Rulef("交接链断裂：本段交出方 %q 必须等于上段接收方 %q", fromPerson, prev.ToPerson)
	}
	if toPerson == prev.ToPerson {
		return domain.Rulef("交接人身份不能重复替代：%q 已连续作为接收方", toPerson)
	}
	if fromPerson == toPerson {
		return domain.Rulef("交接双方不能为同一人")
	}
	return nil
}
