// Package knowledge 工具注册表属性测试
// n9e-2kai: AI 助手模块 - 工具注册属性测试
package knowledge

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// **Feature: ai-assistant-function-calling, Property 1: Tool Registration Completeness**
// **Validates: Requirements 2.1, 2.2**

func TestToolRegistrationCompleteness(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("all enabled tools appear in definitions", prop.ForAll(
		func(numTools int) bool {
			registry := &KnowledgeToolRegistry{
				tools:     make(map[string]*RegisteredTool),
				providers: make(map[int64]Provider),
			}

			// 创建随机数量的工具
			enabledCount := 0
			for i := 0; i < numTools; i++ {
				enabled := i%2 == 0 // 一半启用，一半禁用
				name := string(rune('a' + i))
				registry.tools[name] = &RegisteredTool{
					Name:        name,
					Description: "test tool",
					ProviderID:  1,
					Enabled:     enabled,
				}
				if enabled {
					enabledCount++
				}
			}

			// 获取工具定义
			definitions := registry.GetToolDefinitions()

			// 验证启用的工具数量匹配
			return len(definitions) == enabledCount
		},
		gen.IntRange(0, 20),
	))

	properties.TestingRun(t)
}

// **Feature: ai-assistant-function-calling, Property 2: Multi-Knowledge-Base Tool Generation**
// **Validates: Requirements 2.3**

func TestMultiKnowledgeBaseToolGeneration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("each enabled tool generates unique definition", prop.ForAll(
		func(n int) bool {
			registry := &KnowledgeToolRegistry{
				tools:     make(map[string]*RegisteredTool),
				providers: make(map[int64]Provider),
			}

			// 创建 n 个启用的工具
			for i := 0; i < n; i++ {
				name := string(rune('a' + i))
				registry.tools[name] = &RegisteredTool{
					Name:        name,
					Description: "test tool " + name,
					ProviderID:  int64(i + 1),
					Enabled:     true,
				}
			}

			definitions := registry.GetToolDefinitions()

			// 验证数量匹配
			if len(definitions) != n {
				return false
			}

			// 验证名称唯一
			names := make(map[string]bool)
			for _, def := range definitions {
				if names[def.Function.Name] {
					return false // 重复名称
				}
				names[def.Function.Name] = true
			}

			return true
		},
		gen.IntRange(0, 26), // 最多 26 个工具（a-z）
	))

	properties.TestingRun(t)
}

// **Feature: ai-assistant-function-calling, Property 3: Dynamic Tool Registration**
// **Validates: Requirements 2.5, 2.6**

func TestDynamicToolRegistration(t *testing.T) {
	properties := gopter.NewProperties(gopter.DefaultTestParameters())

	properties.Property("dynamically registered tools appear in definitions", prop.ForAll(
		func(name, description string) bool {
			if name == "" {
				return true // 跳过空名称
			}

			registry := &KnowledgeToolRegistry{
				tools:     make(map[string]*RegisteredTool),
				providers: make(map[int64]Provider),
			}

			// 初始应该没有工具
			if len(registry.GetToolDefinitions()) != 0 {
				return false
			}

			// 动态注册工具
			registry.RegisterTool(&RegisteredTool{
				Name:        name,
				Description: description,
				ProviderID:  1,
				Enabled:     true,
			})

			// 验证工具出现在定义中
			definitions := registry.GetToolDefinitions()
			if len(definitions) != 1 {
				return false
			}

			return definitions[0].Function.Name == name &&
				definitions[0].Function.Description == description
		},
		gen.AlphaString().SuchThat(func(s string) bool { return len(s) > 0 }),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}

func TestIsKnowledgeTool(t *testing.T) {
	registry := &KnowledgeToolRegistry{
		tools:     make(map[string]*RegisteredTool),
		providers: make(map[int64]Provider),
	}

	registry.tools["search_ops_kb"] = &RegisteredTool{
		Name:    "search_ops_kb",
		Enabled: true,
	}

	// 测试存在的工具
	if !registry.IsKnowledgeTool("search_ops_kb") {
		t.Error("expected search_ops_kb to be a knowledge tool")
	}

	// 测试不存在的工具
	if registry.IsKnowledgeTool("unknown_tool") {
		t.Error("expected unknown_tool to not be a knowledge tool")
	}
}

func TestToolDefinitionFormat(t *testing.T) {
	registry := &KnowledgeToolRegistry{
		tools:     make(map[string]*RegisteredTool),
		providers: make(map[int64]Provider),
	}

	registry.tools["test_tool"] = &RegisteredTool{
		Name:        "test_tool",
		Description: "A test tool for searching",
		ProviderID:  1,
		Enabled:     true,
	}

	definitions := registry.GetToolDefinitions()
	if len(definitions) != 1 {
		t.Fatalf("expected 1 definition, got %d", len(definitions))
	}

	def := definitions[0]

	// 验证类型
	if def.Type != "function" {
		t.Errorf("expected type 'function', got '%s'", def.Type)
	}

	// 验证函数名称
	if def.Function.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got '%s'", def.Function.Name)
	}

	// 验证参数结构
	params := def.Function.Parameters
	if params["type"] != "object" {
		t.Errorf("expected parameters type 'object', got '%v'", params["type"])
	}

	props, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("expected properties to be a map")
	}

	queryProp, ok := props["query"].(map[string]interface{})
	if !ok {
		t.Fatal("expected query property to be a map")
	}

	if queryProp["type"] != "string" {
		t.Errorf("expected query type 'string', got '%v'", queryProp["type"])
	}

	required, ok := params["required"].([]string)
	if !ok {
		t.Fatal("expected required to be a string slice")
	}

	if len(required) != 1 || required[0] != "query" {
		t.Errorf("expected required ['query'], got %v", required)
	}
}

func TestFormatResultsForLLM(t *testing.T) {
	// 测试失败响应
	failedResp := &QueryResponse{
		Status: "failed",
		Error:  "connection timeout",
	}
	result := FormatResultsForLLM(failedResp)
	if result != "查询失败: connection timeout" {
		t.Errorf("unexpected result for failed response: %s", result)
	}

	// 测试空结果
	emptyResp := &QueryResponse{
		Status:  "completed",
		Results: []QueryResult{},
	}
	result = FormatResultsForLLM(emptyResp)
	if result != "未找到相关信息" {
		t.Errorf("unexpected result for empty response: %s", result)
	}

	// 测试有答案的响应
	answerResp := &QueryResponse{
		Status: "completed",
		Answer: "The answer is 42",
	}
	result = FormatResultsForLLM(answerResp)
	if result != "The answer is 42" {
		t.Errorf("unexpected result for answer response: %s", result)
	}

	// 测试有结果的响应
	resultsResp := &QueryResponse{
		Status: "completed",
		Results: []QueryResult{
			{Content: "First result", Source: "doc1.md"},
			{Content: "Second result", Source: "doc2.md"},
		},
	}
	result = FormatResultsForLLM(resultsResp)
	if result == "" {
		t.Error("expected non-empty result for results response")
	}
}

func TestUnregisterTool(t *testing.T) {
	registry := &KnowledgeToolRegistry{
		tools:     make(map[string]*RegisteredTool),
		providers: make(map[int64]Provider),
	}

	// 注册工具
	registry.RegisterTool(&RegisteredTool{
		Name:    "test_tool",
		Enabled: true,
	})

	if !registry.IsKnowledgeTool("test_tool") {
		t.Error("tool should be registered")
	}

	// 注销工具
	registry.UnregisterTool("test_tool")

	if registry.IsKnowledgeTool("test_tool") {
		t.Error("tool should be unregistered")
	}
}

func TestGetToolCount(t *testing.T) {
	registry := &KnowledgeToolRegistry{
		tools:     make(map[string]*RegisteredTool),
		providers: make(map[int64]Provider),
	}

	// 初始应该是 0
	if registry.GetToolCount() != 0 {
		t.Error("expected 0 tools initially")
	}

	// 添加启用的工具
	registry.tools["tool1"] = &RegisteredTool{Name: "tool1", Enabled: true}
	registry.tools["tool2"] = &RegisteredTool{Name: "tool2", Enabled: true}
	registry.tools["tool3"] = &RegisteredTool{Name: "tool3", Enabled: false}

	// 应该只计算启用的工具
	if registry.GetToolCount() != 2 {
		t.Errorf("expected 2 enabled tools, got %d", registry.GetToolCount())
	}
}
