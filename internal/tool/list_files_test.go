package tool

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func listRepo(t *testing.T, files []string) *os.Root {
	t.Helper()
	repo := t.TempDir()
	for _, name := range files {
		path := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("内容"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	return root
}

func TestListFiles(t *testing.T) {
	root := listRepo(t, []string{
		"readme.md", "a.go", "internal/tool.go", "internal/nested/z.go",
		".git/config", ".idea/workspace.xml", "internal/.git/config", "internal/.idea/workspace.xml",
	})
	if err := root.Mkdir("empty", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a.go", filepath.Join(root.Name(), "file-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("internal", filepath.Join(root.Name(), "dir-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(root.Name(), "outside-link")); err != nil {
		t.Fatal(err)
	}

	allFiles := []string{"a.go", "internal/nested/z.go", "internal/tool.go", "readme.md"}
	tests := []struct {
		name          string
		path          string
		want          []string
		wantErr       error
		wantPathError bool
	}{
		{name: "default root", want: allFiles},
		{name: "explicit root", path: ".", want: allFiles},
		{name: "subdirectory", path: "internal", want: []string{"internal/nested/z.go", "internal/tool.go"}},
		{name: "normalized path", path: "./internal/", want: []string{"internal/nested/z.go", "internal/tool.go"}},
		{name: "empty directory", path: "empty", want: []string{}},
		{name: "parent traversal", path: "../outside", wantErr: ErrInvalidPath},
		{name: "absolute path", path: root.Name(), wantErr: ErrInvalidPath},
		{name: "file instead of directory", path: "a.go", wantErr: ErrNotDirectory},
		{name: "missing directory", path: "missing", wantErr: fs.ErrNotExist, wantPathError: true},
		{name: "git directory", path: ".git", wantErr: ErrExcludedDirectory},
		{name: "git descendant", path: ".git/hooks", wantErr: ErrExcludedDirectory},
		{name: "idea directory", path: ".idea", wantErr: ErrExcludedDirectory},
		{name: "nested git directory", path: "internal/.git", wantErr: ErrExcludedDirectory},
		{name: "nested idea directory", path: "internal/.idea", wantErr: ErrExcludedDirectory},
		{name: "file symlink", path: "file-link", wantErr: ErrNotDirectory},
		{name: "directory symlink", path: "dir-link", wantErr: ErrNotDirectory},
		{name: "symlink ancestor", path: "dir-link/nested", wantErr: ErrNotDirectory},
		{name: "external symlink", path: "outside-link", wantErr: ErrNotDirectory},
		{name: "external symlink ancestor", path: "outside-link/child", wantErr: ErrNotDirectory},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListFiles(context.Background(), root, tt.path)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ListFiles error = %v, want %v", err, tt.wantErr)
				}
				if tt.wantPathError {
					var pathErr *os.PathError
					if !errors.As(err, &pathErr) {
						t.Fatalf("ListFiles error = %v, want preserved filesystem error", err)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Files == nil || !slices.Equal(got.Files, tt.want) || got.Truncated {
				t.Fatalf("ListFiles = %+v, want files %v without truncation", got, tt.want)
			}
		})
	}
}

func TestListFilesLimit(t *testing.T) {
	for _, count := range []int{0, MaxListFiles, MaxListFiles + 1} {
		t.Run(fmt.Sprintf("%d files", count), func(t *testing.T) {
			names := make([]string, count)
			for i := range names {
				names[i] = fmt.Sprintf("file-%03d.go", i)
			}
			root := listRepo(t, append(slices.Clone(names), ".git/secret", ".idea/workspace.xml"))
			// 排除的目录和链接不占用文件数量，也不会误报截断。
			if err := os.Symlink(".git", filepath.Join(root.Name(), "zz-link")); err != nil {
				t.Fatal(err)
			}
			got, err := ListFiles(context.Background(), root, ".")
			if err != nil {
				t.Fatal(err)
			}
			want := names[:min(count, MaxListFiles)]
			if got.Files == nil || !slices.Equal(got.Files, want) || got.Truncated != (count > MaxListFiles) {
				t.Fatalf("ListFiles = %+v, want %d files, truncated %v", got, len(want), count > MaxListFiles)
			}
		})
	}
}

func TestListFilesHonorsCancellation(t *testing.T) {
	root := listRepo(t, []string{"readme.md"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ListFiles(ctx, root, ".")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ListFiles error = %v, want cancellation", err)
	}
}
