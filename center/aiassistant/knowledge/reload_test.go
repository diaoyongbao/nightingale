// Package knowledge 配置热加载测试
// n9e-2kai: AI 助手模块 - 配置热加载属性测试
package knowledge

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: ai-assistant-function-calling, Property 9: Configuration Hot Reload**
// **Validates: Requirements 7.6, 9.4**

func TestConfigurationHotReload(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("tool changes are reflected after reload", prop.ForAll(
		func(initialCount, addCount int) bool {
			registry := &KnowledgeToolRegistry{
				tools:     make(map[string]*RegisteredTool),
				providers: make(map[int64]Provider),
			}

			// 初始注册工具
			for i := 0; i < initialCount; i++ {
				name := string(rune('a' + i))
				registry.tools[name] = &RegisteredTool{
					Name:    name,
					Enabled: true,
				}
			}

			initialDefs := len(registry.GetToolDefinitions())

			// 模拟热加载：添加新工具
			for i := 0; i < addCount; i++ {
				name := string(rune('A' + i))
				registry.RegisterTool(&RegisteredTool{
					Name:    name,
					Enabled: true,
				})
			}

			finalDefs := len(registry.GetToolDefinitions())

			// 验证工具数量正确更新
			return finalDefs == initialDefs+addCount
		},
		gen.IntRange(0, 10),
		gen.IntRange(0, 10),
	))

	properties.TestingRun(t)
}

func TestProviderRegistrationAfterReload(t *testing.T) {
	// 测试 Provider 注册和注销
	ClearProviders()

	// 注册 Provider
	mockProvider := &mockProvider{name: "test-provider"}
	RegisterProvider("test", mockProvider)

	// 验证注册成功
	provider, err := GetProvider("test")
	if err != nil {
		t.Errorf("expected provider to be registered, got error: %v", err)
	}
	if provider.GetProviderName() != "test-provider" {
		t.Errorf("expected provider name 'test-provider', got '%s'", provider.GetProviderName())
	}

	// 注销 Provider
	UnregisterProvider("test")

	// 验证注销成功
	_, err = GetProvider("test")
	if err == nil {
		t.Error("expected error after unregistering provider")
	}
}

func TestClearProviders(t *testing.T) {
	// 注册多个 Provider
	for i := 0; i < 5; i++ {
		name := string(rune('a' + i))
		RegisterProvider(name, &mockProvider{name: name})
	}

	// 验证注册成功
	providers := ListProviders()
	if len(providers) < 5 {
		t.Errorf("expected at least 5 providers, got %d", len(providers))
	}

	// 清空所有 Provider
	ClearProviders()

	// 验证清空成功
	providers = ListProviders()
	if len(providers) != 0 {
		t.Errorf("expected 0 providers after clear, got %d", len(providers))
	}
}

// mockProvider 用于测试的 mock Provider
type mockProvider struct {
	name string
}

func (m *mockProvider) Query(ctx context.Context, req *QueryRequest) (*QueryResponse, error) {
	return &QueryResponse{Status: "completed"}, nil
}

func (m *mockProvider) Health(ctx context.Context) error {
	return nil
}

func (m *mockProvider) GetProviderName() string {
	return m.name
}

func (m *mockProvider) GetProviderType() string {
	return "mock"
}
