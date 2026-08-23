package rules

import "gowork/internal/domain"

// GradeSeverity 按连续越界次数分级：1-2 轻微，3-4 严重，>=5 危急。
func GradeSeverity(consecutive int) string {
	switch {
	case consecutive >= 5:
		return domain.SeverityCritical
	case consecutive >= 3:
		return domain.SeverityMajor
	default:
		return domain.SeverityMinor
	}
}

// SeverityRank 严重级别权重，用于风险排序。
func SeverityRank(sev string) int {
	switch sev {
	case domain.SeverityCritical:
		return 5
	case domain.SeverityMajor:
		return 3
	case domain.SeverityMinor:
		return 1
	}
	return 0
}

// Escalate 若新级别更严重则返回新级别，否则维持原级别。
func Escalate(current, candidate string) string {
	if SeverityRank(candidate) > SeverityRank(current) {
		return candidate
	}
	return current
}
