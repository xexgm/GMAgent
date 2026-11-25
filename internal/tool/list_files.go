package tool

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const MaxListFiles = 200

type ListFilesResult struct {
	Files     []string `json:"files"`
	Truncated bool     `json:"truncated"`
}

// ListFiles 递归列出目录内的普通文件，返回相对于仓库根目录的路径。
func ListFiles(ctx context.Context, root *os.Root, path string) (ListFilesResult, error) {
	if err := ctx.Err(); err != nil {
		return ListFilesResult{}, err
	}
	if path == "" {
		path = "."
	}
	if !filepath.IsLocal(path) {
		return ListFilesResult{}, ErrInvalidPath
	}
	path = filepath.Clean(path)

	// 检查每一级目录，避免通过符号链接或显式路径进入排除的目录。
	dir := "."
	for _, part := range strings.Split(path, string(filepath.Separator)) {
		if excludedDirectory(part) {
			return ListFilesResult{}, ErrExcludedDirectory
		}
		dir = filepath.Join(dir, part)
		info, err := root.Lstat(dir)
		if err != nil {
			return ListFilesResult{}, fmt.Errorf("检查目录：%w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ListFilesResult{}, ErrNotDirectory
		}
	}

	result := ListFilesResult{Files: make([]string, 0)}
	err := fs.WalkDir(root.FS(), filepath.ToSlash(path), func(path string, entry fs.DirEntry, err error) error {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if excludedDirectory(entry.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if len(result.Files) == MaxListFiles {
			result.Truncated = true
			return fs.SkipAll
		}
		result.Files = append(result.Files, path)
		return nil
	})
	if err != nil {
		return ListFilesResult{}, fmt.Errorf("列出文件：%w", err)
	}
	return result, nil
}

func excludedDirectory(name string) bool {
	return name == ".git" || name == ".idea"
}
