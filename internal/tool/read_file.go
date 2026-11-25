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

// ReadFile 读取仓库内的普通 UTF-8 文本文件；root 由程序确定，模型只能提供相对于它的 path
func ReadFile(root *os.Root, path string) (string, error) {
	if !filepath.IsLocal(path) {
		return "", ErrInvalidPath
	}

	// 先拒绝目录、设备等非普通文件。Root 同时防止符号链接越界。
	info, err := root.Stat(path)
	if err != nil {
		return "", fmt.Errorf("检查文件：%w", err)
	}
	if !info.Mode().IsRegular() {
		return "", ErrNotRegularFile
	}
	if info.Size() > MaxReadBytes {
		return "", ErrFileTooLarge
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
		return "", ErrNotRegularFile
	}

	data, err := io.ReadAll(io.LimitReader(file, MaxReadBytes+1))
	if err != nil {
		return "", fmt.Errorf("读取文件：%w", err)
	}
	if len(data) > MaxReadBytes {
		return "", ErrFileTooLarge
	}
	if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
		return "", ErrNotTextFile
	}
	return string(data), nil
}
