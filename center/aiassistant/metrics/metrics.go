// Package metrics AI 助手 Prometheus 指标
// n9e-2kai: AI 助手模块 - Prometheus 指标
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ChatRequestsTotal 对话请求总数
	ChatRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ai_assistant",
			Name:      "chat_requests_total",
			Help:      "Total number of chat requests",
		},
		[]string{"mode", "status"},
	)

	// ChatRequestDuration 对话请求耗时
	ChatRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ai_assistant",
			Name:      "chat_request_duration_seconds",
			Help:      "Duration of chat requests in seconds",
			Buckets:   []float64{0.1, 0.5, 1, 2, 5, 10, 30},
		},
		[]string{"mode"},
	)

	// ActiveSessions 活跃会话数
	ActiveSessions = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ai_assistant",
			Name:      "active_sessions",
			Help:      "Number of active sessions",
		},
	)

	// ToolCallsTotal 工具调用总数
	ToolCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ai_assistant",
			Name:      "tool_calls_total",
			Help:      "Total number of tool calls",
		},
		[]string{"tool_name", "status"},
	)

	// ConfirmationsPending 待确认操作数
	ConfirmationsPending = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "ai_assistant",
			Name:      "confirmations_pending",
			Help:      "Number of pending confirmations",
		},
	)

	// FileUploadsTotal 文件上传总数
	FileUploadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ai_assistant",
			Name:      "file_uploads_total",
			Help:      "Total number of file uploads",
		},
		[]string{"mime_type", "status"},
	)

	// FileUploadSizeBytes 文件上传大小
	FileUploadSizeBytes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "ai_assistant",
			Name:      "file_upload_size_bytes",
			Help:      "Size of uploaded files in bytes",
			Buckets:   []float64{1024, 10240, 102400, 1048576, 10485760},
		},
	)

	// KnowledgeQueriesTotal 知识库查询总数
	KnowledgeQueriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ai_assistant",
			Name:      "knowledge_queries_total",
			Help:      "Total number of knowledge queries",
		},
		[]string{"provider", "status"},
	)

	// AIModelCallsTotal AI 模型调用总数
	AIModelCallsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "ai_assistant",
			Name:      "ai_model_calls_total",
			Help:      "Total number of AI model calls",
		},
		[]string{"model", "status"},
	)

	// AIModelCallDuration AI 模型调用耗时
	AIModelCallDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "ai_assistant",
			Name:      "ai_model_call_duration_seconds",
			Help:      "Duration of AI model calls in seconds",
			Buckets:   []float64{0.5, 1, 2, 5, 10, 30, 60},
		},
		[]string{"model"},
	)
)

func init() {
	// 注册所有指标
	prometheus.MustRegister(
		ChatRequestsTotal,
		ChatRequestDuration,
		ActiveSessions,
		ToolCallsTotal,
		ConfirmationsPending,
		FileUploadsTotal,
		FileUploadSizeBytes,
		KnowledgeQueriesTotal,
		AIModelCallsTotal,
		AIModelCallDuration,
	)
}

// RecordChatRequest 记录对话请求
func RecordChatRequest(mode, status string, duration float64) {
	ChatRequestsTotal.WithLabelValues(mode, status).Inc()
	ChatRequestDuration.WithLabelValues(mode).Observe(duration)
}

// RecordToolCall 记录工具调用
func RecordToolCall(toolName, status string) {
	ToolCallsTotal.WithLabelValues(toolName, status).Inc()
}

// RecordFileUpload 记录文件上传
func RecordFileUpload(mimeType, status string, size int64) {
	FileUploadsTotal.WithLabelValues(mimeType, status).Inc()
	if status == "success" {
		FileUploadSizeBytes.Observe(float64(size))
	}
}

// RecordKnowledgeQuery 记录知识库查询
func RecordKnowledgeQuery(provider, status string) {
	KnowledgeQueriesTotal.WithLabelValues(provider, status).Inc()
}

// RecordAIModelCall 记录 AI 模型调用
func RecordAIModelCall(model, status string, duration float64) {
	AIModelCallsTotal.WithLabelValues(model, status).Inc()
	AIModelCallDuration.WithLabelValues(model).Observe(duration)
}

// SetActiveSessions 设置活跃会话数
func SetActiveSessions(count int) {
	ActiveSessions.Set(float64(count))
}

// SetConfirmationsPending 设置待确认数
func SetConfirmationsPending(count int) {
	ConfirmationsPending.Set(float64(count))
}
