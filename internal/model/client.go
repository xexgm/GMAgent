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

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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

// Chat 发送一条用户消息，等待模型生成完整回复。
func (c *Client) Chat(ctx context.Context, task string) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"model":    c.modelID,
		"messages": []message{{Role: "user", Content: task}},
		"stream":   false,
		// 第一次接入先关闭深度思考，直接获取文本回复。
		"thinking": map[string]string{"type": "disabled"},
	})
	if err != nil {
		return "", fmt.Errorf("编码模型请求：%w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatEndpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("创建模型请求：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求模型 API：%w", err)
	}
	defer resp.Body.Close()

	// 本次只接收文本回复，限制读取大小，避免异常响应占用过多内存。
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("读取模型响应：%w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("模型 API 返回 HTTP %d：%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result struct {
		Choices []struct {
			Message      message `json:"message"`
			FinishReason string  `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析模型响应：%w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("模型响应中没有 choices")
	}
	if result.Choices[0].FinishReason == "length" {
		return "", fmt.Errorf("模型回复达到输出长度限制，未完整生成，请缩小本次任务")
	}

	reply := result.Choices[0].Message.Content
	if strings.TrimSpace(reply) == "" {
		return "", fmt.Errorf("模型返回了空文本回复")
	}
	return reply, nil
}
