package jobs

// Backoff 计算第 attempts 次失败后的退避秒数（指数退避，上限 1 小时）。
func Backoff(attempts int) int64 {
	sec := int64(1)
	for i := 0; i < attempts; i++ {
		sec *= 2
		if sec >= 3600 {
			return 3600
		}
	}
	return sec
}
