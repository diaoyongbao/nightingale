// Package executor AI 助手并发执行器
// n9e-2kai: AI 助手模块 - 并发工具执行器
package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"

	"golang.org/x/sync/errgroup"
)

// ToolCall 工具调用定义
type ToolCall struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Args     map[string]interface{} `json:"args"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolCallResult 工具调用结果
type ToolCallResult struct {
	ToolCallID string      `json:"tool_call_id"`
	ToolName   string      `json:"tool_name"`
	Result     interface{} `json:"result"`
	Error      error       `json:"error,omitempty"`
	Duration   int64       `json:"duration_ms"`
	Success    bool        `json:"success"`
}

// ToolExecutor 工具执行函数类型
type ToolExecutor func(ctx context.Context, toolCall ToolCall) (interface{}, error)

// ConcurrentExecutor 并发执行器
type ConcurrentExecutor struct {
	maxConcurrency int
	config         *models.ConcurrentConfig
	mu             sync.RWMutex
	appCtx         *ctx.Context
}

// NewConcurrentExecutor 创建并发执行器
func NewConcurrentExecutor(appCtx *ctx.Context) (*ConcurrentExecutor, error) {
	config, err := models.GetConcurrentConfig(appCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to load concurrent config: %w", err)
	}

	return &ConcurrentExecutor{
		maxConcurrency: config.MaxConcurrency,
		config:         config,
		appCtx:         appCtx,
	}, nil
}


// ExecuteAll 并发执行多个工具调用
func (e *ConcurrentExecutor) ExecuteAll(ctx context.Context, toolCalls []ToolCall, executor ToolExecutor) []ToolCallResult {
	if len(toolCalls) == 0 {
		return nil
	}

	results := make([]ToolCallResult, len(toolCalls))
	maxConcurrency := e.GetMaxConcurrency()

	// 使用 semaphore 限制并发数
	sem := make(chan struct{}, maxConcurrency)

	g, gCtx := errgroup.WithContext(ctx)

	for i, tc := range toolCalls {
		i, tc := i, tc // 捕获循环变量

		g.Go(func() error {
			// 获取信号量
			select {
			case sem <- struct{}{}:
			case <-gCtx.Done():
				results[i] = ToolCallResult{
					ToolCallID: tc.ID,
					ToolName:   tc.Name,
					Error:      gCtx.Err(),
					Success:    false,
				}
				return nil
			}
			defer func() { <-sem }() // 释放信号量

			start := time.Now()
			result, err := executor(gCtx, tc)
			duration := time.Since(start).Milliseconds()

			results[i] = ToolCallResult{
				ToolCallID: tc.ID,
				ToolName:   tc.Name,
				Result:     result,
				Error:      err,
				Duration:   duration,
				Success:    err == nil,
			}

			return nil // 不中断其他调用
		})
	}

	g.Wait()
	return results
}

// ExecuteOne 执行单个工具调用
func (e *ConcurrentExecutor) ExecuteOne(ctx context.Context, toolCall ToolCall, executor ToolExecutor) ToolCallResult {
	start := time.Now()
	result, err := executor(ctx, toolCall)
	duration := time.Since(start).Milliseconds()

	return ToolCallResult{
		ToolCallID: toolCall.ID,
		ToolName:   toolCall.Name,
		Result:     result,
		Error:      err,
		Duration:   duration,
		Success:    err == nil,
	}
}

// GetMaxConcurrency 获取最大并发数
func (e *ConcurrentExecutor) GetMaxConcurrency() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.maxConcurrency <= 0 {
		return 5 // 默认值
	}
	return e.maxConcurrency
}

// SetMaxConcurrency 设置最大并发数
func (e *ConcurrentExecutor) SetMaxConcurrency(max int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.maxConcurrency = max
}

// Reload 重新加载配置
func (e *ConcurrentExecutor) Reload() error {
	config, err := models.GetConcurrentConfig(e.appCtx)
	if err != nil {
		return err
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	e.config = config
	e.maxConcurrency = config.MaxConcurrency

	return nil
}

// GetConfig 获取当前配置
func (e *ConcurrentExecutor) GetConfig() *models.ConcurrentConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.config
}

// AggregateResults 聚合结果
func AggregateResults(results []ToolCallResult) (successes []ToolCallResult, failures []ToolCallResult) {
	for _, r := range results {
		if r.Success {
			successes = append(successes, r)
		} else {
			failures = append(failures, r)
		}
	}
	return
}

// HasFailures 检查是否有失败
func HasFailures(results []ToolCallResult) bool {
	for _, r := range results {
		if !r.Success {
			return true
		}
	}
	return false
}

// GetTotalDuration 获取总耗时
func GetTotalDuration(results []ToolCallResult) int64 {
	var total int64
	for _, r := range results {
		if r.Duration > total {
			total = r.Duration // 并发执行，取最长时间
		}
	}
	return total
}
