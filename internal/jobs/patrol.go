package jobs

import (
	"context"

	"gowork/internal/service"
)

// PatrolHandler 环境巡检作业：处理未处理采样，生成/升级异常事件。
func PatrolHandler(anomalies *service.AnomalyService) Handler {
	return func(ctx context.Context) error {
		_, err := anomalies.Patrol(ctx)
		return err
	}
}
