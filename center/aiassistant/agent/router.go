// Package agent 动态路由器
// n9e-2kai: AI 助手模块 - 动态 Agent 路由
package agent

import (
	"context"
	"regexp"
	"strings"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ai"
	"github.com/toolkits/pkg/logger"
)

// Router 动态路由器
type Router struct {
	loader   *Loader
	aiClient ai.AIClient
}

// NewRouter 创建动态路由器
func NewRouter(loader *Loader, aiClient ai.AIClient) *Router {
	return &Router{
		loader:   loader,
		aiClient: aiClient,
	}
}

// RouteResult 路由结果
type RouteResult struct {
	Agent      *AgentConfig
	IsMention  bool   // 是否通过 @Mention 直连
	MatchType  string // keyword/llm/default/mention
	CleanQuery string // 清理后的查询（移除 @Mention 部分）
}

// mentionRegex @Mention 正则
var mentionRegex = regexp.MustCompile(`@(\w+)\s*`)

// Route 路由决策
func (r *Router) Route(ctx context.Context, message string) *RouteResult {
	// L0: 检查 @Mention
	if result := r.checkMention(message); result != nil {
		return result
	}

	// L1: 关键词匹配
	if agent := r.loader.MatchAgentByKeywords(message); agent != nil {
		return &RouteResult{
			Agent:      agent,
			MatchType:  "keyword",
			CleanQuery: message,
		}
	}

	// L2: LLM 路由（如果 Agent 数量较多）
	if r.aiClient != nil && r.loader.GetAgentCount() > 5 {
		if agent := r.llmRoute(ctx, message); agent != nil {
			return &RouteResult{
				Agent:      agent,
				MatchType:  "llm",
				CleanQuery: message,
			}
		}
	}

	// 默认使用通用 Agent
	generalAgent := r.loader.GetSystemAgent(models.AIAgentNameGeneral)
	if generalAgent == nil {
		// 如果没有通用 Agent，尝试知识库 Agent
		generalAgent = r.loader.GetSystemAgent(models.AIAgentNameKnowledge)
	}

	return &RouteResult{
		Agent:      generalAgent,
		MatchType:  "default",
		CleanQuery: message,
	}
}

// checkMention 检查 @Mention
func (r *Router) checkMention(message string) *RouteResult {
	matches := mentionRegex.FindStringSubmatch(message)
	if len(matches) < 2 {
		return nil
	}

	agentName := matches[1]
	agent := r.loader.GetAgent(agentName)
	if agent == nil {
		// 尝试模糊匹配
		agent = r.fuzzyMatchAgent(agentName)
	}

	if agent == nil {
		return nil
	}

	// 移除 @Mention 部分
	cleanQuery := mentionRegex.ReplaceAllString(message, "")
	cleanQuery = strings.TrimSpace(cleanQuery)

	return &RouteResult{
		Agent:      agent,
		IsMention:  true,
		MatchType:  "mention",
		CleanQuery: cleanQuery,
	}
}

// fuzzyMatchAgent 模糊匹配 Agent
func (r *Router) fuzzyMatchAgent(name string) *AgentConfig {
	lowerName := strings.ToLower(name)
	for _, agent := range r.loader.GetAllAgents() {
		if strings.Contains(strings.ToLower(agent.Name), lowerName) {
			return agent
		}
	}
	return nil
}

// llmRoute 使用 LLM 进行路由决策
func (r *Router) llmRoute(ctx context.Context, message string) *AgentConfig {
	routerAgent := r.loader.GetSystemAgent(models.AIAgentNameRouter)
	if routerAgent == nil {
		return nil
	}

	// 构建候选 Agent 列表
	var candidates []string
	for _, agent := range r.loader.GetAgentsForMention() {
		candidates = append(candidates, agent.Name+": "+agent.Description)
	}

	if len(candidates) == 0 {
		return nil
	}

	// 构建路由 Prompt
	prompt := "用户输入: " + message + "\n\n可用专家列表:\n"
	for i, c := range candidates {
		prompt += string(rune('1'+i)) + ". " + c + "\n"
	}
	prompt += "\n请返回最合适的专家名称（只返回名称，不要其他内容）。如果都不匹配返回 \"sys_general\"。"

	// 调用 LLM
	resp, err := r.aiClient.ChatCompletion(ctx, &ai.ChatCompletionRequest{
		Model:        routerAgent.Model,
		Temperature:  routerAgent.Temperature,
		MaxTokens:    100,
		SystemPrompt: routerAgent.SystemPrompt,
		Messages: []ai.Message{
			{Role: "user", Content: prompt},
		},
	})

	if err != nil {
		logger.Warningf("LLM routing failed: %v", err)
		return nil
	}

	if len(resp.Choices) == 0 {
		return nil
	}

	// 解析 LLM 返回的 Agent 名称
	agentName := ""
	if content, ok := resp.Choices[0].Message.Content.(string); ok {
		agentName = strings.TrimSpace(content)
	}

	if agentName == "" {
		return nil
	}

	return r.loader.GetAgent(agentName)
}

// ParseMention 解析消息中的 @Mention
func ParseMention(message string) (agentName string, cleanMessage string, hasMention bool) {
	matches := mentionRegex.FindStringSubmatch(message)
	if len(matches) < 2 {
		return "", message, false
	}

	agentName = matches[1]
	cleanMessage = strings.TrimSpace(mentionRegex.ReplaceAllString(message, ""))
	return agentName, cleanMessage, true
}
