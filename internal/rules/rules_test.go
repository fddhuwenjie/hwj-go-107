package rules_test

import (
	"testing"

	"gowork/internal/domain"
	"gowork/internal/rules"
)

func sample(at int64, temp, hum float64) domain.EnvSample {
	return domain.EnvSample{SampledAt: at, Temperature: temp, Humidity: hum}
}

var rule = domain.ThresholdRuleVersion{
	TempMin: 14, TempMax: 24, HumidityMin: 45, HumidityMax: 60, ConsecutiveBreach: 2,
}

// TestBreach 阈值越界判定。
func TestBreach(t *testing.T) {
	if rules.Breach(sample(0, 20, 50), rule) {
		t.Fatal("合格采样不应越界")
	}
	for _, s := range []domain.EnvSample{
		sample(0, 13.9, 50), sample(0, 24.1, 50), sample(0, 20, 44.9), sample(0, 20, 60.1),
	} {
		if !rules.Breach(s, rule) {
			t.Fatalf("应判定越界: %+v", s)
		}
	}
}

// TestGradeSeverity 异常分级。
func TestGradeSeverity(t *testing.T) {
	cases := map[int]string{1: "minor", 2: "minor", 3: "major", 4: "major", 5: "critical", 9: "critical"}
	for n, want := range cases {
		if got := rules.GradeSeverity(n); got != want {
			t.Fatalf("连续 %d 次应为 %s，实际 %s", n, want, got)
		}
	}
}

// TestContinuousQualified 前置窗口连续合格判定。
func TestContinuousQualified(t *testing.T) {
	from, to, gap := int64(0), int64(7200), int64(1800)

	// 合格序列
	ok := []domain.EnvSample{sample(0, 20, 50), sample(1800, 20, 50), sample(3600, 20, 50), sample(5400, 20, 50), sample(7200, 20, 50)}
	if res := rules.ContinuousQualified(ok, rule, from, to, gap); !res.Qualified {
		t.Fatalf("应合格: %s", res.Reason)
	}
	// 空窗口
	if res := rules.ContinuousQualified(nil, rule, from, to, gap); res.Qualified {
		t.Fatal("无采样不应合格")
	}
	// 越界
	bad := append([]domain.EnvSample{}, ok...)
	bad[2] = sample(3600, 30, 50)
	if res := rules.ContinuousQualified(bad, rule, from, to, gap); res.Qualified {
		t.Fatal("越界不应合格")
	}
	// 间隔过大
	sparse := []domain.EnvSample{sample(0, 20, 50), sample(3600, 20, 50), sample(7200, 20, 50)}
	if res := rules.ContinuousQualified(sparse, rule, from, to, gap); res.Qualified {
		t.Fatal("间隔过大不应合格")
	}
	// 迟到数据被忽略
	late := append([]domain.EnvSample{}, ok...)
	late[2].Late = true
	if res := rules.ContinuousQualified(late, rule, from, to, gap); res.Qualified {
		t.Fatal("忽略迟到数据后间隔为 3600 > gap，不应合格")
	}
}

// TestHandoverOrderValid 交接有序性规则。
func TestHandoverOrderValid(t *testing.T) {
	prev := &domain.PackageHandover{Seq: 1, FromPerson: "A", ToPerson: "B", HandedAt: 100}
	if err := rules.HandoverOrderValid(prev, "B", "C", 200); err != nil {
		t.Fatalf("正常交接应通过: %v", err)
	}
	if err := rules.HandoverOrderValid(prev, "B", "C", 100); err == nil {
		t.Fatal("时间未递增应拒绝")
	}
	if err := rules.HandoverOrderValid(prev, "X", "C", 200); err == nil {
		t.Fatal("交接链断裂应拒绝")
	}
	if err := rules.HandoverOrderValid(prev, "B", "B", 200); err == nil {
		t.Fatal("身份重复替代应拒绝")
	}
	if err := rules.HandoverOrderValid(nil, "A", "A", 100); err == nil {
		t.Fatal("首段自交自接应拒绝")
	}
}

// TestStrictestRule 最严格规则合并。
func TestStrictestRule(t *testing.T) {
	r2 := domain.ThresholdRuleVersion{TempMin: 16, TempMax: 22, HumidityMin: 50, HumidityMax: 55, ConsecutiveBreach: 3}
	merged, ok := rules.StrictestRule([]domain.ThresholdRuleVersion{rule, r2})
	if !ok {
		t.Fatal("应可合并")
	}
	if merged.TempMin != 16 || merged.TempMax != 22 || merged.HumidityMin != 50 || merged.HumidityMax != 55 || merged.ConsecutiveBreach != 2 {
		t.Fatalf("合并结果应为区间交集: %+v", merged)
	}
	if _, ok := rules.StrictestRule(nil); ok {
		t.Fatal("空规则集不应可合并")
	}
}
