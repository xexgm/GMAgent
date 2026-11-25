package tool

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"
)

const MaxReadBytes = 64 * 1024

// ReadFile 读取仓库内的普通 UTF-8 文本文件。
// root 由程序确定，模型只能提供相对于它的 path。
func ReadFile(root *os.Root, path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", fmt.Errorf("path 必须是仓库内的相对路径")
	}

	// 先拒绝目录、设备等非普通文件。Root 同时防止符号链接越界。
	info, err := root.Stat(path)
	if err != nil {
		return "", fmt.Errorf("检查文件：%w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("只允许读取普通文件")
	}
	if info.Size() > MaxReadBytes {
		return "", fmt.Errorf("文件超过 64 KB 读取限制")
	}

	file, err := root.Open(path)
	if err != nil {
		return "", fmt.Errorf("打开文件：%w", err)
	}
	defer file.Close()

	info, err = file.Stat()
	if err != nil {
		return "", fmt.Errorf("检查已打开文件：%w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("只允许读取普通文件")
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxReadBytes+1))
	if err != nil {
		return "", fmt.Errorf("读取文件：%w", err)
	}
	if len(data) > MaxReadBytes {
		return "", fmt.Errorf("文件超过 64 KB 读取限制")
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", fmt.Errorf("只支持 UTF-8 文本文件")
	}
	return string(data), nil
}
