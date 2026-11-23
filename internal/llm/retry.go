package llm

import (
	"context"
	"errors"
	"time"
)

// RetryableError 携带 HTTP 状态码与是否可重试。
type RetryableError struct {
	Status    int
	Retryable bool
	Message   string
}

func (e *RetryableError) Error() string { return e.Message }

// ClassifyStatus 按 T207 策略表判断错误是否可重试与最大次数。
func ClassifyStatus(status int) (retryable bool, maxAttempts int) {
	switch {
	case status == 429:
		return true, 5
	case status >= 500 && status <= 599:
		return true, 3
	case status == 400 || status == 401 || status == 403:
		return false, 1
	default:
		return true, 4 // 连接失败 / EOF 按 4 次退避
	}
}

// backoffBase 可被测试调小；生产默认 1s。
var backoffBase = time.Second

// WithRetry 用 1s/2s/4s/8s 指数退避，最多 maxAttempts 次；
// 退避必须可被 ctx 取消，不可用 time.Sleep。
func WithRetry(ctx context.Context, maxAttempts int, fn func() error) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var err error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		var re *RetryableError
		if errors.As(err, &re) && !re.Retryable {
			return err
		}
		if attempt == maxAttempts-1 {
			return err
		}
		d := backoffBase << attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
		}
	}
	return err
}
