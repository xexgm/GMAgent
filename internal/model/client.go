package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const DefaultModel = "doubao-seed-2-0-mini-260428"

const chatEndpoint = "https://ark.cn-beijing.volces.com/api/v3/chat/completions"

type Client struct {
	apiKey     string
	modelID    string
	httpClient *http.Client
}

func NewClient(apiKey, modelID string) (*Client, error) {
	apiKey = strings.TrimSpace(apiKey)
	modelID = strings.TrimSpace(modelID)
	if apiKey == "" {
		return nil, fmt.Errorf("请先设置环境变量 ARK_API_KEY")
	}
	if modelID == "" {
		return nil, fmt.Errorf("模型 ID 不能为空")
	}

	return &Client{
		apiKey:     apiKey,
		modelID:    modelID,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}, nil
}

// Chat 发送历史消息和工具定义，返回文本回复或工具调用请求。
func (c *Client) Chat(ctx context.Context, messages []Message, tools []ToolDefinition) (Message, error) {
	payload, err := json.Marshal(chatRequest{
		Model:    c.modelID,
		Messages: messages,
		Tools:    tools,
		// 第一次接入先关闭深度思考，直接获取文本回复。
		Thinking: thinking{Type: "disabled"},
	})
	if err != nil {
		return Message{}, fmt.Errorf("编码模型请求：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatEndpoint, bytes.NewReader(payload))
	if err != nil {
		return Message{}, fmt.Errorf("创建模型请求：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Message{}, fmt.Errorf("请求模型 API：%w", err)
	}
	defer resp.Body.Close()

	// 限制响应大小，读取额外一个字节用于检测是否超限。
	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return Message{}, fmt.Errorf("读取模型响应：%w", err)
	}
	if len(body) > maxResponseBytes {
		return Message{}, fmt.Errorf("模型响应超过 1 MB 限制")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Message{}, fmt.Errorf("模型 API 返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result chatResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return Message{}, fmt.Errorf("解析模型响应：%w", err)
	}
	if len(result.Choices) == 0 {
		return Message{}, fmt.Errorf("模型响应中没有 choices")
	}
	if result.Choices[0].FinishReason == "length" {
		return Message{}, fmt.Errorf("模型回复达到输出长度限制，未完整生成，请缩小本次任务")
	}
	if result.Choices[0].FinishReason == "content_filter" {
		return Message{}, fmt.Errorf("模型回复被内容过滤，未完整生成")
	}

	reply := result.Choices[0].Message
	if reply.Role != "assistant" {
		return Message{}, fmt.Errorf("模型返回了异常消息角色：%q", reply.Role)
	}
	if len(reply.ToolCalls) == 0 && strings.TrimSpace(reply.Content) == "" {
		return Message{}, fmt.Errorf("模型没有返回文本或工具调用")
	}
	return reply, nil
}
