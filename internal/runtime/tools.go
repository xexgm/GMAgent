package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/xexgm/GMAgent/internal/model"
	"github.com/xexgm/GMAgent/internal/tool"
)

type pathArguments struct {
	Path json.RawMessage `json:"path"`
}

func readFileDefinition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.FunctionDefinition{
			Name:        "read_file",
			Description: "读取仓库内一个普通 UTF-8 文本文件，最多 64 KB；返回完整内容或错误。",
			Parameters: model.ParameterSchema{
				Type: "object",
				Properties: map[string]model.ParameterProperty{
					"path": {Type: "string", Description: "相对于仓库根目录的文件路径，例如 readme.md"},
				},
				Required: []string{"path"},
			},
		},
	}
}

func listFilesDefinition() model.ToolDefinition {
	return model.ToolDefinition{
		Type: "function",
		Function: model.FunctionDefinition{
			Name:        "list_files",
			Description: "递归列出仓库目录内的普通文件，跳过 .git、.idea 和符号链接；返回仓库相对路径 files 和 truncated，最多 200 个文件。截断时可指定子目录继续列出。",
			Parameters: model.ParameterSchema{
				Type: "object",
				Properties: map[string]model.ParameterProperty{
					"path": {Type: "string", Description: "相对于仓库根目录的目录路径；省略或为空时列出整个仓库，例如 internal"},
				},
				Required: []string{},
			},
		},
	}
}

// 第二步注册：把工具名对应到本地函数。
func executeTool(ctx context.Context, root *os.Root, call model.ToolCall) (string, error) {
	name := call.Function.Name
	if call.Type != "function" || (name != "read_file" && name != "list_files") {
		return "", fmt.Errorf("不支持的工具：%q", name)
	}
	path, err := decodePathArguments(name, call.Function.Arguments)
	if err != nil {
		return "", err
	}
	if name == "read_file" {
		return tool.ReadFile(root, path)
	}
	result, err := tool.ListFiles(ctx, root, path)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("编码 list_files 结果：%w", err)
	}
	return string(data), nil
}

func decodePathArguments(name, arguments string) (string, error) {
	var args *pathArguments
	decoder := json.NewDecoder(strings.NewReader(arguments))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&args); err != nil {
		return "", fmt.Errorf("解析 %s 参数：%w", name, err)
	}
	if args == nil {
		return "", fmt.Errorf("%s 参数必须是 JSON 对象", name)
	}
	// 参数只允许一个 JSON 对象，拒绝后面多余的内容。
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", fmt.Errorf("%s 参数包含多余内容", name)
	}
	if len(args.Path) == 0 && name == "list_files" {
		return "", nil
	}
	if len(args.Path) == 0 || string(args.Path) == "null" {
		return "", fmt.Errorf("%s 的 path 参数必须是字符串", name)
	}
	var path string
	if err := json.Unmarshal(args.Path, &path); err != nil {
		return "", fmt.Errorf("解析 %s 的 path 参数：%w", name, err)
	}
	return path, nil
}
