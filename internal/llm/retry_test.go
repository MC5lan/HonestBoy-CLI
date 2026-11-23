package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status   int
		retry    bool
		attempts int
	}{
		{0, true, 4}, {429, true, 5}, {500, true, 3}, {503, true, 3},
		{400, false, 1}, {401, false, 1}, {403, false, 1},
	}
	for _, tc := range cases {
		retry, attempts := ClassifyStatus(tc.status)
		if retry != tc.retry || attempts != tc.attempts {
			t.Fatalf("status %d: retry=%v attempts=%d", tc.status, retry, attempts)
		}
	}
}

func TestWithRetrySucceedsAfterFailures(t *testing.T) {
	old := backoffBase
	backoffBase = time.Millisecond
	defer func() { backoffBase = old }()

	calls := 0
	err := WithRetry(context.Background(), 3, func() error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWithRetryStopsOnNonRetryable(t *testing.T) {
	old := backoffBase
	backoffBase = time.Millisecond
	defer func() { backoffBase = old }()

	calls := 0
	err := WithRetry(context.Background(), 5, func() error {
		calls++
		return &RetryableError{Status: 401, Retryable: false, Message: "bad key"}
	})
	if err == nil || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}

func TestWithRetryContextCancel(t *testing.T) {
	old := backoffBase
	backoffBase = time.Hour
	defer func() { backoffBase = old }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WithRetry(ctx, 4, func() error { return errors.New("fail") })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v", err)
	}
}
