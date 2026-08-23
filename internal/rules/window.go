package rules

import (
	"gowork/internal/domain"
)

// WindowResult 前置窗口判定结果。
type WindowResult struct {
	// Qualified 是否连续合格。
	Qualified bool
	// Reason 不合格原因（合格时为空）。
	Reason string
	// SampleCount 窗口内采样数。
	SampleCount int
	// BreachCount 窗口内越界采样数。
	BreachCount int
}

// ContinuousQualified 判定某单元在 [from, to] 前置窗口内采样是否连续合格：
//  1. 窗口内（含边界）至少有一条非迟到采样；
//  2. 窗口内无任何越界采样；
//  3. 覆盖度：最早一条不晚于 from+maxGap，最晚一条不早于 to-maxGap，
//     且相邻采样间隔不超过 maxGap。
//
// samples 必须按 sampled_at 升序，rule 为该单元适用的启用阈值规则。
func ContinuousQualified(samples []domain.EnvSample, rule domain.ThresholdRuleVersion, from, to, maxGap int64) WindowResult {
	res := WindowResult{}
	var in []domain.EnvSample
	for _, s := range samples {
		if s.Late || s.SampledAt < from || s.SampledAt > to {
			continue
		}
		in = append(in, s)
		if Breach(s, rule) {
			res.BreachCount++
		}
	}
	res.SampleCount = len(in)
	if len(in) == 0 {
		res.Reason = "前置窗口内无有效环境采样"
		return res
	}
	if res.BreachCount > 0 {
		res.Reason = "前置窗口内存在越界采样"
		return res
	}
	if in[0].SampledAt > from+maxGap {
		res.Reason = "窗口起始段缺少采样覆盖"
		return res
	}
	if in[len(in)-1].SampledAt < to-maxGap {
		res.Reason = "窗口末段缺少采样覆盖"
		return res
	}
	for i := 1; i < len(in); i++ {
		if in[i].SampledAt-in[i-1].SampledAt > maxGap {
			res.Reason = "窗口内采样间隔超过允许最大值"
			return res
		}
	}
	res.Qualified = true
	return res
}
