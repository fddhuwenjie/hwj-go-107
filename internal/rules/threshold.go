// Package rules 存放纯函数业务规则：阈值判定、异常分级、前置窗口连续合格判定。
package rules

import (
	"gowork/internal/domain"
)

// Breach 判定一条采样是否越过规则阈值。
func Breach(s domain.EnvSample, r domain.ThresholdRuleVersion) bool {
	return s.Temperature < r.TempMin || s.Temperature > r.TempMax ||
		s.Humidity < r.HumidityMin || s.Humidity > r.HumidityMax
}

// ConsecutiveBreaches 统计按采样时间升序序列末尾的连续越界次数。
// samples 必须已按 sampled_at 升序排列。
func ConsecutiveBreaches(samples []domain.EnvSample, r domain.ThresholdRuleVersion) int {
	n := 0
	for i := len(samples) - 1; i >= 0; i-- {
		if samples[i].Late {
			continue // 迟到数据仅存档，不参与判定
		}
		if Breach(samples[i], r) {
			n++
		} else {
			break
		}
	}
	return n
}
