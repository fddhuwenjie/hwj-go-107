package httpx_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"gowork/internal/httpx"
	"gowork/internal/testenv"
)

// newTestServer 基于测试环境构造 HTTP 服务器。
func newTestServer(t *testing.T, env *testenv.Env) *httptest.Server {
	t.Helper()
	srv := &httpx.Server{
		Artifacts: env.Artifact,
		Env:       env.Env,
		Anomalies: env.Anomaly,
		Loans:     env.Loan,
		Checks:    env.Check,
		Handovers: env.Handover,
		Returns:   env.Return,
		Queries:   env.Query,
		Repo:      env.Repo,
		DB:        env.DB,
		Clock:     env.Clock,
		Log:       slog.New(slog.NewJSONHandler(io.Discard, nil)),
	}
	ts := httptest.NewServer(httpx.NewHandler(srv))
	t.Cleanup(ts.Close)
	return ts
}

func post(t *testing.T, url string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func get(t *testing.T, url string) (int, map[string]any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

// TestHealthz 健康检查。
func TestHealthz(t *testing.T) {
	env := testenv.New(t)
	ts := newTestServer(t, env)
	code, body := get(t, ts.URL+"/healthz")
	if code != http.StatusOK || body["status"] != "ok" || body["db"] != "up" {
		t.Fatalf("健康检查异常: %d %+v", code, body)
	}
}

// TestHTTPLoanChain 通过 HTTP 走通登记到审批。
func TestHTTPLoanChain(t *testing.T) {
	env := testenv.New(t)
	ts := newTestServer(t, env)

	code, lv := post(t, ts.URL+"/api/v1/preservation-levels", map[string]any{"code": "LV-1", "name": "一级"})
	if code != http.StatusCreated {
		t.Fatalf("创建等级失败: %d %+v", code, lv)
	}
	levelID := int64(lv["id"].(float64))

	code, rv := post(t, ts.URL+"/api/v1/threshold-rules", map[string]any{
		"level_id": levelID, "temp_min": 14, "temp_max": 24, "humidity_min": 45, "humidity_max": 60, "consecutive_breach": 2,
	})
	if code != http.StatusCreated {
		t.Fatalf("创建规则失败: %d %+v", code, rv)
	}
	ruleID := int64(rv["id"].(float64))
	code, _ = post(t, ts.URL+"/api/v1/threshold-rules/"+itoa(ruleID)+"/activate", map[string]any{})
	if code != http.StatusOK {
		t.Fatalf("启用规则失败: %d", code)
	}

	code, unit := post(t, ts.URL+"/api/v1/storage-units", map[string]any{
		"code": "WH-A", "name": "甲字库房", "kind": "warehouse",
	})
	if code != http.StatusCreated {
		t.Fatalf("创建库房失败: %d %+v", code, unit)
	}
	unitID := int64(unit["id"].(float64))

	code, sensor := post(t, ts.URL+"/api/v1/sensors", map[string]any{"code": "S-1", "storage_unit_id": unitID})
	if code != http.StatusCreated {
		t.Fatalf("创建传感器失败: %d %+v", code, sensor)
	}
	sensorID := int64(sensor["id"].(float64))

	code, art := post(t, ts.URL+"/api/v1/artifacts", map[string]any{
		"code": "WJ-0001", "name": "宋刻本", "level_id": levelID,
	})
	if code != http.StatusCreated {
		t.Fatalf("登记藏品失败: %d %+v", code, art)
	}
	artID := int64(art["id"].(float64))
	version := int64(art["version"].(float64))

	code, art2 := post(t, ts.URL+"/api/v1/artifacts/"+itoa(artID)+"/assign-location", map[string]any{
		"storage_unit_id": unitID, "version": version,
	})
	if code != http.StatusOK || art2["status"] != "stored" {
		t.Fatalf("分配库位失败: %d %+v", code, art2)
	}

	// 环境采样覆盖前置窗口
	now := env.Now()
	from := now - int64(env.Cfg.PreLoanWindow.Seconds())
	for ts0 := from; ts0 <= now; ts0 += 1800 {
		code, s := post(t, ts.URL+"/api/v1/env-samples", map[string]any{
			"sensor_id": sensorID, "temperature": 20, "humidity": 50, "sampled_at": ts0,
		})
		if code != http.StatusCreated {
			t.Fatalf("采样失败: %d %+v", code, s)
		}
	}

	code, loan := post(t, ts.URL+"/api/v1/loans", map[string]any{
		"code": "LN-1", "borrower": "省博物馆", "start_at": now + 86400, "end_at": now + 30*86400,
		"artifact_ids": []int64{artID},
	})
	if code != http.StatusCreated {
		t.Fatalf("创建借展失败: %d %+v", code, loan)
	}
	loanID := int64(loan["id"].(float64))
	loanVer := int64(loan["version"].(float64))

	code, sub := post(t, ts.URL+"/api/v1/loans/"+itoa(loanID)+"/submit", map[string]any{"version": loanVer})
	if code != http.StatusOK {
		t.Fatalf("提交失败: %d %+v", code, sub)
	}
	code, appr := post(t, ts.URL+"/api/v1/loans/"+itoa(loanID)+"/approve", map[string]any{"version": loanVer + 1, "reviewer": "赵六"})
	if code != http.StatusOK || appr["status"] != "approved" {
		t.Fatalf("审批失败: %d %+v", code, appr)
	}

	// 分页查询藏品
	code, list := get(t, ts.URL+"/api/v1/artifacts?limit=10")
	if code != http.StatusOK {
		t.Fatalf("分页查询失败: %d", code)
	}
	items, ok := list["items"].([]any)
	if !ok {
		t.Fatalf("分页响应缺少 items: %+v", list)
	}
	if len(items) != 1 {
		t.Fatalf("分页结果异常: %+v", list)
	}
}

// TestUnifiedError 统一错误格式。
func TestUnifiedError(t *testing.T) {
	env := testenv.New(t)
	ts := newTestServer(t, env)

	// 404
	code, body := get(t, ts.URL+"/api/v1/artifacts/999")
	if code != http.StatusNotFound {
		t.Fatalf("期望 404，实际 %d", code)
	}
	errObj := body["error"].(map[string]any)
	if errObj["code"] != "not_found" || errObj["message"] == "" {
		t.Fatalf("错误格式不统一: %+v", body)
	}

	// 400 参数错误
	code, body = post(t, ts.URL+"/api/v1/artifacts", map[string]any{"code": "", "name": ""})
	if code != http.StatusBadRequest {
		t.Fatalf("期望 400，实际 %d %+v", code, body)
	}

	// 路由不存在
	code, _ = get(t, ts.URL+"/api/v1/nonexistent")
	if code != http.StatusNotFound {
		t.Fatalf("期望 404 路由，实际 %d", code)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}

// TestHTTPSensorUnitBinding 验证传感器始终绑定请求指定的存储单元：
// 给二号展柜注册的传感器，按二号展柜筛选必须能查到它（且其 storage_unit_id 仍为二号展柜），
// 按一号库房筛选必须查不到它。环境采样也必须记到二号展柜。
// 覆盖 单元读取→传感器创建→按单元筛选 的跨实体关联。
func TestHTTPSensorUnitBinding(t *testing.T) {
	env := testenv.New(t)
	ts := newTestServer(t, env)

	// 一号库房
	code, wh := post(t, ts.URL+"/api/v1/storage-units", map[string]any{
		"code": "WH-A", "name": "一号库房", "kind": "warehouse",
	})
	if code != http.StatusCreated {
		t.Fatalf("创建库房失败: %d %+v", code, wh)
	}
	whID := int64(wh["id"].(float64))

	// 一号库房也注册一个传感器，确保筛选能在多传感器间区分
	code, whSensor := post(t, ts.URL+"/api/v1/sensors", map[string]any{"code": "WH-A-S1", "storage_unit_id": whID})
	if code != http.StatusCreated {
		t.Fatalf("创建一号库房传感器失败: %d %+v", code, whSensor)
	}

	// 二号展柜
	code, sc := post(t, ts.URL+"/api/v1/storage-units", map[string]any{
		"code": "SC-B", "name": "二号展柜", "kind": "showcase",
	})
	if code != http.StatusCreated {
		t.Fatalf("创建展柜失败: %d %+v", code, sc)
	}
	scID := int64(sc["id"].(float64))
	if scID == whID {
		t.Fatalf("单元主键冲突")
	}

	// 给二号展柜注册传感器
	code, sensor := post(t, ts.URL+"/api/v1/sensors", map[string]any{"code": "SC-B-S1", "storage_unit_id": scID})
	if code != http.StatusCreated {
		t.Fatalf("创建传感器失败: %d %+v", code, sensor)
	}
	if int64(sensor["storage_unit_id"].(float64)) != scID {
		t.Fatalf("创建返回的传感器绑定到错误单元: 期望 %d 实际 %v", scID, sensor["storage_unit_id"])
	}
	scSensorID := int64(sensor["id"].(float64))

	// 按二号展柜筛选：必须能查到该传感器，且其 storage_unit_id 仍是二号展柜
	code, listSC := get(t, ts.URL+"/api/v1/sensors?storage_unit_id="+itoa(scID))
	if code != http.StatusOK {
		t.Fatalf("查询展柜传感器失败: %d", code)
	}
	scItems, ok := listSC["items"].([]any)
	if !ok {
		t.Fatalf("展柜筛选响应缺少 items: %+v", listSC)
	}
	if len(scItems) != 1 {
		t.Fatalf("二号展柜应恰好 1 个传感器，实际 %d", len(scItems))
	}
	row := scItems[0].(map[string]any)
	if int64(row["id"].(float64)) != scSensorID {
		t.Fatalf("按二号展柜筛选返回了错误传感器: 期望 %d 实际 %v", scSensorID, row["id"])
	}
	if int64(row["storage_unit_id"].(float64)) != scID {
		t.Fatalf("筛选返回传感器绑定单元被篡改: 期望 %d 实际 %v", scID, row["storage_unit_id"])
	}

	// 按一号库房筛选：必须查不到二号展柜的传感器
	code, listWH := get(t, ts.URL+"/api/v1/sensors?storage_unit_id="+itoa(whID))
	if code != http.StatusOK {
		t.Fatalf("查询库房传感器失败: %d", code)
	}
	whItems, _ := listWH["items"].([]any)
	for _, it := range whItems {
		if int64(it.(map[string]any)["id"].(float64)) == scSensorID {
			t.Fatalf("二号展柜传感器不应出现在一号库房筛选结果中")
		}
	}

	// 环境采样必须记到二号展柜
	now := env.Now()
	code, sample := post(t, ts.URL+"/api/v1/env-samples", map[string]any{
		"sensor_id": scSensorID, "temperature": 20, "humidity": 50, "sampled_at": now,
	})
	if code != http.StatusCreated {
		t.Fatalf("采样失败: %d %+v", code, sample)
	}
	if int64(sample["storage_unit_id"].(float64)) != scID {
		t.Fatalf("采样记到错误单元: 期望 %d 实际 %v", scID, sample["storage_unit_id"])
	}

	// 按二号展柜查询采样，必须能查到
	code, listSamples := get(t, ts.URL+"/api/v1/env-samples?storage_unit_id="+itoa(scID))
	if code != http.StatusOK {
		t.Fatalf("查询展柜采样失败: %d", code)
	}
	sampleItems, _ := listSamples["items"].([]any)
	if len(sampleItems) != 1 {
		t.Fatalf("二号展柜应恰好 1 条采样，实际 %d", len(sampleItems))
	}
}
