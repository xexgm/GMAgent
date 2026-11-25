# GMAgent

学习目标：通过实践 coding agent，理解 Agent 工程并积累解决问题的经验。

做一个真实任务 → 查看执行轨迹 → 找到失败原因 → 调整上下文、工具或规则 → 用同一组评测验证是否改善。

当前进度：

- 已接入方舟 Chat Completions API，通过 `tools` 描述自定义 `list_files` 和 `read_file` 工具。
- 已实现简单 Runtime：请求模型 → 执行工具 → 回传结果 → 再次请求模型；没有工具请求时输出最终回复。
- 本次任务的历史消息、工具调用和结果保存在内存中，通过 `tool_call_id` 关联；工具错误也会回传给模型。
- 模型可以先用 `list_files` 发现仓库文件，再用 `read_file` 读取内容；单测覆盖目录排除、符号链接、数量限制、路径边界、参数校验、错误恢复和循环限制。

配置好环境变量 `ARK_API_KEY` 后，在项目根目录运行：

```bash
go run ./cmd/agent --task "先列出仓库文件，再找到并读取 Go 程序入口，解释启动流程。"
```

`list_files` 的目录参数 `path` 可省略，默认递归列出启动目录内的普通文件，跳过 `.git`、`.idea` 和符号链接。返回的 `files` 始终是相对于仓库根目录的路径；最多返回 200 个文件，`truncated` 表示是否还有未列出的文件，可指定子目录继续查看。

`read_file` 读取普通 UTF-8 文本文件，单文件最多 64 KB；两个工具的路径都限于启动目录内。任务最多调用模型 5 次、单轮最多执行 4 次工具调用，总超时 2 分钟。终端会显示工具调用过程。

代码阅读顺序：`internal/tool/list_files.go` 和 `read_file.go`（工具实现）→ `internal/runtime/tools.go`（注册与执行）→ `internal/runtime/runtime.go`（循环）→ `internal/model/types.go` 和 `client.go`（接口数据与请求）。本地验证：`go test ./...`。

下一步：加入代码搜索、修改文件和运行测试工具，逐步完成一次真实 bug 修复。
