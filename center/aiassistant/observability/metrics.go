// Package observability AI 助手可观测化
// n9e-2kai: AI 助手模块 - Prometheus 指标
package observability

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal 请求计数器
	RequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_requests_total",
			Help: "Total number of AI assistant requests",
		},
		[]string{"status", "model", "tool"},
	)

	// RequestDuration 请求延迟直方图
	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_assistant_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"model", "tool"},
	)

	// TokensUsed Token 使用量
	TokensUsed = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_tokens_total",
			Help: "Total tokens used",
		},
		[]string{"model", "type"}, // type: prompt/completion
	)

	// CacheHits 缓存命中计数
	CacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_cache_hits_total",
			Help: "Cache hit count",
		},
		[]string{"tool", "hit"}, // hit: true/false
	)

	// RateLimitHits 速率限制触发计数
	RateLimitHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_rate_limit_hits_total",
			Help: "Rate limit hit count",
		},
		[]string{"user_id", "busi_group_id"},
	)

	// ToolCallDuration 工具调用延迟
	ToolCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_assistant_tool_call_duration_seconds",
			Help:    "Tool call duration in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"tool", "status"},
	)

	// RetryAttempts 重试次数
	RetryAttempts = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "ai_assistant_retry_attempts",
			Help:    "Number of retry attempts",
			Buckets: []float64{0, 1, 2, 3, 4, 5},
		},
		[]string{"tool"},
	)

	// ConcurrentExecutions 并发执行数
	ConcurrentExecutions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "ai_assistant_concurrent_executions",
			Help: "Current number of concurrent tool executions",
		},
	)

	// CostTotal 成本总计
	CostTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_cost_total",
			Help: "Total cost in USD",
		},
		[]string{"model"},
	)

	// ErrorsTotal 错误计数
	ErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "ai_assistant_errors_total",
			Help: "Total number of errors",
		},
		[]string{"type", "code"},
	)
)


// RecordRequest 记录请求
func RecordRequest(status, model, tool string) {
	RequestsTotal.WithLabelValues(status, model, tool).Inc()
}

// RecordRequestDuration 记录请求延迟
func RecordRequestDuration(model, tool string, durationSeconds float64) {
	RequestDuration.WithLabelValues(model, tool).Observe(durationSeconds)
}

// RecordTokens 记录 Token 使用量
func RecordTokens(model string, promptTokens, completionTokens int) {
	TokensUsed.WithLabelValues(model, "prompt").Add(float64(promptTokens))
	TokensUsed.WithLabelValues(model, "completion").Add(float64(completionTokens))
}

// RecordCacheHit 记录缓存命中
func RecordCacheHit(tool string, hit bool) {
	hitStr := "false"
	if hit {
		hitStr = "true"
	}
	CacheHits.WithLabelValues(tool, hitStr).Inc()
}

// RecordRateLimitHit 记录速率限制触发
func RecordRateLimitHit(userID string, busiGroupID int64) {
	RateLimitHits.WithLabelValues(userID, fmt.Sprintf("%d", busiGroupID)).Inc()
}

// RecordToolCallDuration 记录工具调用延迟
func RecordToolCallDuration(tool, status string, durationSeconds float64) {
	ToolCallDuration.WithLabelValues(tool, status).Observe(durationSeconds)
}

// RecordRetryAttempts 记录重试次数
func RecordRetryAttempts(tool string, attempts int) {
	RetryAttempts.WithLabelValues(tool).Observe(float64(attempts))
}

// RecordConcurrentExecutions 记录并发执行数
func RecordConcurrentExecutions(count int) {
	ConcurrentExecutions.Set(float64(count))
}

// IncrementConcurrentExecutions 增加并发执行数
func IncrementConcurrentExecutions() {
	ConcurrentExecutions.Inc()
}

// DecrementConcurrentExecutions 减少并发执行数
func DecrementConcurrentExecutions() {
	ConcurrentExecutions.Dec()
}

// RecordCost 记录成本
func RecordCost(model string, cost float64) {
	CostTotal.WithLabelValues(model).Add(cost)
}

// RecordError 记录错误
func RecordError(errorType, errorCode string) {
	ErrorsTotal.WithLabelValues(errorType, errorCode).Inc()
}
