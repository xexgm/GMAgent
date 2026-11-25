package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/xexgm/GMAgent/internal/model"
	"github.com/xexgm/GMAgent/internal/tool"
)

type modelFunc func(context.Context, []model.Message, []model.ToolDefinition) (model.Message, error)

func (f modelFunc) Chat(ctx context.Context, messages []model.Message, tools []model.ToolDefinition) (model.Message, error) {
	return f(ctx, messages, tools)
}

func testRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := root.WriteFile("readme.md", []byte("项目说明"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("empty.txt", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readCall(id, path string) model.ToolCall {
	return model.ToolCall{
		ID:   id,
		Type: "function",
		Function: model.FunctionCall{
			Name:      "read_file",
			Arguments: fmt.Sprintf(`{"path":%q}`, path),
		},
	}
}

func TestRunReturnsToolResultsWithMatchingIDs(t *testing.T) {
	calls := 0
	client := modelFunc(func(_ context.Context, messages []model.Message, tools []model.ToolDefinition) (model.Message, error) {
		calls++
		if len(tools) != 2 || tools[0].Function.Name != "read_file" || tools[1].Function.Name != "list_files" {
			t.Fatal("both tool definitions must be sent on every request")
		}
		if calls == 1 {
			return model.Message{
				Role:      "assistant",
				ToolCalls: []model.ToolCall{readCall("readme-call", "readme.md"), readCall("empty-call", "empty.txt")},
			}, nil
		}
		if len(messages) != 5 {
			t.Fatalf("got %d messages, want user/system + assistant + two results", len(messages))
		}
		if messages[2].Role != "assistant" || len(messages[2].ToolCalls) != 2 {
			t.Fatal("assistant tool-call message is missing")
		}
		if messages[3].Role != "tool" || messages[3].ToolCallID != "readme-call" || messages[3].Content != "项目说明" {
			t.Fatal("readme result does not match its tool call")
		}
		if messages[4].Role != "tool" || messages[4].ToolCallID != "empty-call" || messages[4].Content != "" {
			t.Fatal("empty file result does not match its tool call")
		}
		return model.Message{Role: "assistant", Content: "已阅读项目说明"}, nil
	})
	var trace bytes.Buffer
	got, err := Run(context.Background(), client, "读取项目说明", testRepo(t), &trace)
	if err != nil || got != "已阅读项目说明" || calls != 2 {
		t.Fatalf("Run = %q, %v, calls %d", got, err, calls)
	}
	if !strings.Contains(trace.String(), "成功读取 0 字节") {
		t.Fatal("empty file should be reported as a successful read")
	}
}

func TestRunListsFilesBeforeReading(t *testing.T) {
	calls := 0
	client := modelFunc(func(_ context.Context, messages []model.Message, tools []model.ToolDefinition) (model.Message, error) {
		calls++
		if len(tools) != 2 || tools[0].Function.Name != "read_file" || tools[1].Function.Name != "list_files" {
			t.Fatal("both tool definitions must be sent on every request")
		}
		switch calls {
		case 1:
			return model.Message{
				Role: "assistant",
				ToolCalls: []model.ToolCall{{
					ID:   "discover",
					Type: "function",
					Function: model.FunctionCall{
						Name:      "list_files",
						Arguments: `{}`,
					},
				}},
			}, nil
		case 2:
			if len(messages) != 4 || messages[2].ToolCalls[0].ID != "discover" {
				t.Fatal("list call was not preserved in message history")
			}
			last := messages[len(messages)-1]
			if last.Role != "tool" || last.ToolCallID != "discover" {
				t.Fatal("list result does not match its tool call")
			}
			var result tool.ListFilesResult
			if err := json.Unmarshal([]byte(last.Content), &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Files) != 2 || result.Truncated {
				t.Fatalf("unexpected file listing: %+v", result)
			}
			for _, path := range result.Files {
				if strings.HasSuffix(path, ".md") {
					return model.Message{Role: "assistant", ToolCalls: []model.ToolCall{readCall("read-discovered", path)}}, nil
				}
			}
			t.Fatal("list_files did not discover the project documentation")
		case 3:
			if len(messages) != 6 {
				t.Fatalf("got %d messages, want history of list and read calls", len(messages))
			}
			last := messages[len(messages)-1]
			if last.Role != "tool" || last.ToolCallID != "read-discovered" || last.Content != "项目说明" {
				t.Fatal("discovered file content was not returned to the model")
			}
			return model.Message{Role: "assistant", Content: "已找到并读取项目说明"}, nil
		default:
			t.Fatal("unexpected extra model call")
		}
		return model.Message{}, nil
	})
	var trace bytes.Buffer
	got, err := Run(context.Background(), client, "找到并阅读项目说明", testRepo(t), &trace)
	if err != nil || got != "已找到并读取项目说明" || calls != 3 {
		t.Fatalf("Run = %q, %v, calls %d", got, err, calls)
	}
	if !strings.Contains(trace.String(), `"files"`) || !strings.Contains(trace.String(), "成功读取") {
		t.Fatal("trace should show both file discovery and reading")
	}
}

func TestRunCanRecoverFromToolError(t *testing.T) {
	calls := 0
	client := modelFunc(func(_ context.Context, messages []model.Message, _ []model.ToolDefinition) (model.Message, error) {
		calls++
		switch calls {
		case 1:
			return model.Message{Role: "assistant", ToolCalls: []model.ToolCall{readCall("missing", "missing.txt")}}, nil
		case 2:
			last := messages[len(messages)-1]
			if last.ToolCallID != "missing" || !strings.Contains(last.Content, "工具执行失败") {
				t.Fatal("tool error was not returned to the model")
			}
			return model.Message{Role: "assistant", ToolCalls: []model.ToolCall{readCall("retry", "readme.md")}}, nil
		default:
			last := messages[len(messages)-1]
			if last.ToolCallID != "retry" || last.Content != "项目说明" {
				t.Fatal("retry result was not returned to the model")
			}
			return model.Message{Role: "assistant", Content: "读取成功"}, nil
		}
	})
	got, err := Run(context.Background(), client, "读取说明", testRepo(t), nil)
	if err != nil || got != "读取成功" || calls != 3 {
		t.Fatalf("Run = %q, %v, calls %d", got, err, calls)
	}
}

func TestRunStopsAtModelCallLimit(t *testing.T) {
	calls := 0
	client := modelFunc(func(context.Context, []model.Message, []model.ToolDefinition) (model.Message, error) {
		calls++
		return model.Message{
			Role:      "assistant",
			ToolCalls: []model.ToolCall{readCall(fmt.Sprintf("call-%d", calls), "readme.md")},
		}, nil
	})
	_, err := Run(context.Background(), client, "一直读取", testRepo(t), nil)
	if err == nil || calls != maxModelCalls {
		t.Fatalf("Run error = %v, calls %d, want stop at %d", err, calls, maxModelCalls)
	}
}

func TestRunHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := modelFunc(func(context.Context, []model.Message, []model.ToolDefinition) (model.Message, error) {
		t.Fatal("canceled task should not call the model")
		return model.Message{}, nil
	})
	_, err := Run(ctx, client, "读取说明", testRepo(t), nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want cancellation", err)
	}
}

func TestExecuteToolRejectsInvalidCalls(t *testing.T) {
	root, err := os.OpenRoot(testRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	cases := []model.ToolCall{
		{Type: "function", Function: model.FunctionCall{Name: "unknown", Arguments: `{}`}},
		{Type: "other", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":"readme.md"}`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":1}`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":"readme.md","extra":true}`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":"readme.md"} {}`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `null`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{}`}},
		{Type: "function", Function: model.FunctionCall{Name: "read_file", Arguments: `{"path":null}`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `null`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `[]`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `"."`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{"path":null}`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{"path":1}`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{"path":".","extra":true}`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{} {}`}},
		{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: `{} trailing`}},
	}
	for _, call := range cases {
		if _, err := executeTool(context.Background(), root, call); err == nil {
			t.Fatalf("unexpected success for invalid tool request: %+v", call)
		}
	}
}

func TestExecuteListFilesDefaultsToRoot(t *testing.T) {
	root, err := os.OpenRoot(testRepo(t))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	for _, arguments := range []string{`{}`, `{"path":""}`, `{"path":"."}`} {
		call := model.ToolCall{Type: "function", Function: model.FunctionCall{Name: "list_files", Arguments: arguments}}
		got, err := executeTool(context.Background(), root, call)
		if err != nil {
			t.Fatal(err)
		}
		if got != `{"files":["empty.txt","readme.md"],"truncated":false}` {
			t.Fatalf("executeTool with %s = %s, want both repository files", arguments, got)
		}
	}
}
