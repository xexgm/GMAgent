package tool

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFile(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "repo")
	if err := os.Mkdir(repoDir, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"readme.md": "项目说明\n",
		"empty.txt": "",
		"limit.txt": strings.Repeat("a", MaxReadBytes),
		"large.txt": strings.Repeat("a", MaxReadBytes+1),
		"nul.bin":   "text\x00binary",
		"bad.bin":   "\xff",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(repoDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(base, "outside.txt")
	if err := os.WriteFile(outside, []byte("仓库外内容"), 0o600); err != nil {
		t.Fatal(err)
	}
	// 仓库内相对链接可以读取，指向仓库外的链接必须被拒绝。
	if err := os.Symlink("readme.md", filepath.Join(repoDir, "inside-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../outside.txt", filepath.Join(repoDir, "outside-link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "text", path: "readme.md", want: files["readme.md"]},
		{name: "empty file", path: "empty.txt"},
		{name: "exact size limit", path: "limit.txt", want: files["limit.txt"]},
		{name: "internal symlink", path: "inside-link", want: files["readme.md"]},
		{name: "missing file", path: "missing.txt", wantErr: true},
		{name: "empty path", path: "", wantErr: true},
		{name: "directory", path: ".", wantErr: true},
		{name: "parent traversal", path: "../outside.txt", wantErr: true},
		{name: "absolute path", path: outside, wantErr: true},
		{name: "external symlink", path: "outside-link", wantErr: true},
		{name: "too large", path: "large.txt", wantErr: true},
		{name: "binary with NUL", path: "nul.bin", wantErr: true},
		{name: "invalid UTF-8", path: "bad.bin", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadFile(root, tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadFile error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("ReadFile returned unexpected content: length %d, want %d", len(got), len(tt.want))
			}
		})
	}
}
