// Package retry AI 助手重试处理器
// n9e-2kai: AI 助手模块 - 重试处理器（指数退避）
package retry

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
)

// RetryableError 可重试错误
type RetryableError struct {
	Err        error         `json:"error"`
	Retryable  bool          `json:"retryable"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
}

func (e *RetryableError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "retryable error"
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// NewRetryableError 创建可重试错误
func NewRetryableError(err error, retryable bool) *RetryableError {
	return &RetryableError{
		Err:       err,
		Retryable: retryable,
	}
}

// RetryHandler 重试处理器
type RetryHandler struct {
	config *models.RetryConfig
	mu     sync.RWMutex
	appCtx *ctx.Context
}

// RetryResult 重试结果
type RetryResult struct {
	Result       interface{}   `json:"result"`
	Error        error         `json:"error,omitempty"`
	Attempts     int           `json:"attempts"`
	TotalLatency time.Duration `json:"total_latency"`
	History      []RetryAttempt `json:"history"`
}

// RetryAttempt 重试尝试记录
type RetryAttempt struct {
	Attempt  int           `json:"attempt"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
	Backoff  time.Duration `json:"backoff"`
}

// NewRetryHandler 创建重试处理器
func NewRetryHandler(appCtx *ctx.Context) (*RetryHandler, error) {
	config, err := models.GetRetryConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load retry config: %w", err)
	}

	return &RetryHandler{
		config: config,
		appCtx: appCtx,
	}, nil
}


// Execute 带重试执行函数
func (r *RetryHandler) Execute(ctx context.Context, fn func() (interface{}, error)) *RetryResult {
	result := &RetryResult{
		History: make([]RetryAttempt, 0),
	}

	startTime := time.Now()
	maxRetries := r.GetMaxRetries()

	for attempt := 0; attempt <= maxRetries; attempt++ {
		attemptStart := time.Now()
		res, err := fn()
		attemptDuration := time.Since(attemptStart)

		result.Attempts = attempt + 1

		if err == nil {
			result.Result = res
			result.TotalLatency = time.Since(startTime)
			return result
		}

		// 记录尝试历史
		backoff := r.CalculateBackoff(attempt)
		attemptRecord := RetryAttempt{
			Attempt:  attempt + 1,
			Error:    err.Error(),
			Duration: attemptDuration,
			Backoff:  backoff,
		}
		result.History = append(result.History, attemptRecord)

		// 检查是否可重试
		if retryErr, ok := err.(*RetryableError); ok && !retryErr.Retryable {
			result.Error = err
			result.TotalLatency = time.Since(startTime)
			return result
		}

		// 如果还有重试机会，等待退避时间
		if attempt < maxRetries {
			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				result.TotalLatency = time.Since(startTime)
				return result
			case <-time.After(backoff):
			}
		} else {
			result.Error = fmt.Errorf("max retries exceeded: %w", err)
		}
	}

	result.TotalLatency = time.Since(startTime)
	return result
}

// CalculateBackoff 计算退避时间
func (r *RetryHandler) CalculateBackoff(attempt int) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	initialBackoff := time.Duration(r.config.InitialBackoff) * time.Millisecond
	maxBackoff := time.Duration(r.config.MaxBackoff) * time.Millisecond
	multiplier := r.config.Multiplier

	backoff := float64(initialBackoff) * math.Pow(multiplier, float64(attempt))
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	return time.Duration(backoff)
}

// GetMaxRetries 获取最大重试次数
func (r *RetryHandler) GetMaxRetries() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config.MaxRetries
}

// Reload 重新加载配置
func (r *RetryHandler) Reload() error {
	config, err := models.GetRetryConfig(r.appCtx)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config

	return nil
}

// GetConfig 获取当前配置
func (r *RetryHandler) GetConfig() *models.RetryConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	if retryErr, ok := err.(*RetryableError); ok {
		return retryErr.Retryable
	}

	// 默认认为是可重试的
	return true
}

// WrapRetryable 包装为可重试错误
func WrapRetryable(err error) error {
	return NewRetryableError(err, true)
}

// WrapNonRetryable 包装为不可重试错误
func WrapNonRetryable(err error) error {
	return NewRetryableError(err, false)
}
