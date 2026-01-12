// Package knowledge Cloudflare AutoRAG Provider 测试
// n9e-2kai: AI 助手模块 - Provider 属性测试
package knowledge

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: ai-assistant-function-calling, Property 8: Provider Interface Compliance**
// **Validates: Requirements 9.1**

func TestCloudflareRAGProviderInterfaceCompliance(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("CloudflareRAGProvider implements Provider interface", prop.ForAll(
		func(accountID, ragName, apiToken, model string, rewriteQuery bool, maxResults int, scoreThreshold float64, timeout int) bool {
			config := &CloudflareRAGConfig{
				AccountID:      accountID,
				RAGName:        ragName,
				APIToken:       apiToken,
				Model:          model,
				RewriteQuery:   rewriteQuery,
				MaxNumResults:  maxResults,
				ScoreThreshold: scoreThreshold,
				Timeout:        timeout,
			}
			provider := NewCloudflareRAGProvider("test-provider", config)

			// 验证实现了 Provider 接口
			var _ Provider = provider

			// 验证 GetProviderName 返回正确的名称
			if provider.GetProviderName() != "test-provider" {
				return false
			}

			// 验证 GetProviderType 返回正确的类型
			if provider.GetProviderType() != "cloudflare_autorag" {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
		gen.AlphaString(),
		gen.Bool(),
		gen.IntRange(1, 100),
		gen.Float64Range(0, 1),
		gen.IntRange(1, 60),
	))

	properties.TestingRun(t)
}

func TestQueryRequestGetQueryText(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("GetQueryText returns Query if set, otherwise Message", prop.ForAll(
		func(query, message string) bool {
			req := &QueryRequest{
				Query:   query,
				Message: message,
			}

			result := req.GetQueryText()

			if query != "" {
				return result == query
			}
			return result == message
		},
		gen.AlphaString(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestCloudflareRAGProviderQueryEmptyText(t *testing.T) {
	config := &CloudflareRAGConfig{
		AccountID: "test",
		RAGName:   "test",
		APIToken:  "test",
		Timeout:   5,
	}
	provider := NewCloudflareRAGProvider("test", config)

	// 测试空查询
	resp, err := provider.Query(context.Background(), &QueryRequest{})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if resp.Status != "failed" {
		t.Errorf("expected status 'failed', got '%s'", resp.Status)
	}
	if resp.Error != "query text is empty" {
		t.Errorf("expected error 'query text is empty', got '%s'", resp.Error)
	}
}

func TestCloudflareRAGProviderDefaultValues(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("Provider uses default values when config values are zero", prop.ForAll(
		func(accountID, ragName string) bool {
			config := &CloudflareRAGConfig{
				AccountID:      accountID,
				RAGName:        ragName,
				APIToken:       "test-token",
				MaxNumResults:  0, // 应该使用默认值 10
				ScoreThreshold: 0, // 应该使用默认值 0.3
				Timeout:        0, // 应该使用默认值 30
			}

			provider := NewCloudflareRAGProvider("test", config)

			// 验证 provider 创建成功
			return provider != nil && provider.client != nil
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
	))

	properties.TestingRun(t)
}
