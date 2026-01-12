// Package observability AI 助手可观测化
// n9e-2kai: AI 助手模块 - OpenTelemetry 链路追踪
package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "ai-assistant"

var tracer = otel.Tracer(tracerName)

// StartSpan 开始一个新的 Span
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// StartChatSpan 开始 Chat 请求 Span
func StartChatSpan(ctx context.Context, sessionID, userID string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "ai.chat",
		trace.WithAttributes(
			attribute.String("ai.session_id", sessionID),
			attribute.String("ai.user_id", userID),
		),
	)
}

// StartLLMSpan 开始 LLM 调用 Span
func StartLLMSpan(ctx context.Context, model string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "ai.llm.call",
		trace.WithAttributes(
			attribute.String("llm.model", model),
		),
	)
}

// StartToolSpan 开始工具调用 Span
func StartToolSpan(ctx context.Context, toolName string) (context.Context, trace.Span) {
	return tracer.Start(ctx, "ai.tool.call",
		trace.WithAttributes(
			attribute.String("tool.name", toolName),
		),
	)
}

// RecordLLMCall 记录 LLM 调用信息
func RecordLLMCall(span trace.Span, model string, promptTokens, completionTokens int, durationMs int64) {
	span.SetAttributes(
		attribute.String("llm.model", model),
		attribute.Int("llm.prompt_tokens", promptTokens),
		attribute.Int("llm.completion_tokens", completionTokens),
		attribute.Int("llm.total_tokens", promptTokens+completionTokens),
		attribute.Int64("llm.duration_ms", durationMs),
	)

	// 同时更新 Prometheus 指标
	RecordTokens(model, promptTokens, completionTokens)
}

// RecordToolCall 记录工具调用信息
func RecordToolCall(span trace.Span, toolName string, success bool, durationMs int64) {
	status := "success"
	if !success {
		status = "error"
	}

	span.SetAttributes(
		attribute.String("tool.name", toolName),
		attribute.Bool("tool.success", success),
		attribute.String("tool.status", status),
		attribute.Int64("tool.duration_ms", durationMs),
	)

	// 同时更新 Prometheus 指标
	RecordToolCallDuration(toolName, status, float64(durationMs)/1000.0)
}

// RecordCacheAccess 记录缓存访问
func RecordCacheAccess(span trace.Span, toolName string, hit bool) {
	span.SetAttributes(
		attribute.String("cache.tool", toolName),
		attribute.Bool("cache.hit", hit),
	)

	// 同时更新 Prometheus 指标
	RecordCacheHit(toolName, hit)
}

// RecordRateLimit 记录速率限制
func RecordRateLimit(span trace.Span, userID string, busiGroupID int64, allowed bool) {
	span.SetAttributes(
		attribute.String("ratelimit.user_id", userID),
		attribute.Int64("ratelimit.busi_group_id", busiGroupID),
		attribute.Bool("ratelimit.allowed", allowed),
	)

	if !allowed {
		RecordRateLimitHit(userID, busiGroupID)
	}
}

// RecordRetry 记录重试信息
func RecordRetry(span trace.Span, toolName string, attempts int, success bool) {
	span.SetAttributes(
		attribute.String("retry.tool", toolName),
		attribute.Int("retry.attempts", attempts),
		attribute.Bool("retry.success", success),
	)

	// 同时更新 Prometheus 指标
	RecordRetryAttempts(toolName, attempts)
}

// SetSpanError 设置 Span 错误
func SetSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetSpanOK 设置 Span 成功
func SetSpanOK(span trace.Span) {
	span.SetStatus(codes.Ok, "")
}

// AddSpanEvent 添加 Span 事件
func AddSpanEvent(span trace.Span, name string, attrs ...attribute.KeyValue) {
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// GetTraceID 从 context 获取 trace ID
func GetTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// GetSpanID 从 context 获取 span ID
func GetSpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasSpanID() {
		return span.SpanContext().SpanID().String()
	}
	return ""
}
