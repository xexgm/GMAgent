package tool

import "errors"

// 固定业务错误在这里定义，调用方通过 errors.Is 判断类别；底层文件系统错误仍在操作位置通过 %w 包装，保留原始原因。
var (
	ErrInvalidPath       = errors.New("path 必须是仓库内的相对路径")
	ErrNotDirectory      = errors.New("只允许列出非符号链接目录")
	ErrExcludedDirectory = errors.New("不允许列出 .git 或 .idea 目录")
	ErrNotRegularFile    = errors.New("只允许读取普通文件")
	ErrFileTooLarge      = errors.New("文件超过 64 KB 读取限制")
	ErrNotTextFile       = errors.New("只支持 UTF-8 文本文件")
)
