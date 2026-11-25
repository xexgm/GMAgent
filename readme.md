# GMAgent

学习目标：通过实践 coding agent，理解 Agent 工程并积累解决问题的经验。

做一个真实任务 → 查看执行轨迹 → 找到失败原因 → 调整上下文、工具或规则 → 用同一组评测验证是否改善。

当前进度：

- 已接入方舟 Chat Completions API，通过 `tools` 描述自定义 `read_file` 工具。
- 已实现简单 Runtime：请求模型 → 执行工具 → 回传结果 → 再次请求模型；没有工具请求时输出最终回复。
- 本次任务的历史消息、工具调用和结果保存在内存中，通过 `tool_call_id` 关联；工具错误也会回传给模型。
- 已真实跑通读取 `readme.md` 并回答的流程，单测覆盖路径边界、参数校验、错误恢复和循环限制。

配置好环境变量 `ARK_API_KEY` 后，在项目根目录运行：

```bash
go run ./cmd/agent --task "读取 readme.md，说明项目的当前进度和下一步"
```

读取范围为启动目录内的普通 UTF-8 文本文件，单文件最多 64 KB；任务最多调用模型 5 次、单轮最多执行 4 次工具调用，总超时 2 分钟。终端会显示工具调用过程。

代码阅读顺序：`internal/tool/read_file.go`（读取）→ `internal/runtime/runtime.go`（注册与循环）→ `internal/model/types.go` 和 `client.go`（接口数据与请求）。本地验证：`go test ./...`。

下一步：加入代码搜索、修改文件和运行测试工具，逐步完成一次真实 bug 修复。
