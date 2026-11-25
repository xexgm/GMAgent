package model

// Message 既用于保存对话历史，也用于解析模型返回的消息。
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // API 返回的是 JSON 字符串，需要再次解析。
}

type ToolDefinition struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  ParameterSchema `json:"parameters"`
}

type ParameterSchema struct {
	Type                 string                       `json:"type"`
	Properties           map[string]ParameterProperty `json:"properties"`
	Required             []string                     `json:"required"`
	AdditionalProperties bool                         `json:"additionalProperties"`
}

type ParameterProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type chatRequest struct {
	Model    string           `json:"model"`
	Messages []Message        `json:"messages"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	Stream   bool             `json:"stream"`
	Thinking thinking         `json:"thinking"`
}

type thinking struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []choice `json:"choices"`
}

type choice struct {
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}
