package model

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestChatSendsToolsAndParsesToolCall(t *testing.T) {
	client, err := NewClient("test-key", DefaultModel)
	if err != nil {
		t.Fatal(err)
	}
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodPost || req.URL.String() != chatEndpoint {
			t.Fatal("unexpected model API request")
		}
		if req.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("API key is not in the Authorization header")
		}
		var payload chatRequest
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Messages) != 1 || payload.Messages[0].Content != "读取说明" {
			t.Fatal("user message is missing")
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "read_file" {
			t.Fatal("tool definition is missing")
		}
		if payload.Thinking.Type != "disabled" || payload.Stream {
			t.Fatal("expected non-streaming response with thinking disabled")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(
				`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"readme.md\"}"}}]}}]}`,
			)),
		}, nil
	})
	reply, err := client.Chat(context.Background(),
		[]Message{{Role: "user", Content: "读取说明"}},
		[]ToolDefinition{{Type: "function", Function: FunctionDefinition{Name: "read_file"}}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(reply.ToolCalls) != 1 || reply.ToolCalls[0].ID != "call-1" ||
		reply.ToolCalls[0].Function.Arguments != `{"path":"readme.md"}` {
		t.Fatalf("unexpected tool call: %+v", reply)
	}
}
