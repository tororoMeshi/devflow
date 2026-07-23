package artifact

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadFile(t *testing.T) {
	root := t.TempDir()
	write := func(path, content string) {
		t.Helper()
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("out/report.md", "artifact")
	got, err := ReadFile(root, "out/report.md")
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != "sha256:c7c5c1d70c5dec4416ab6158afd0b223ef40c29b1dc1f97ed9428b94d4cadb1c" || got.Size != 8 {
		t.Fatalf("evidence = %#v", got)
	}
	write("empty", "")
	empty, err := ReadFile(root, "empty")
	if err != nil || empty.Size != 0 || empty.Digest != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty = %#v, %v", empty, err)
	}
	write("large", strings.Repeat("x", 2<<20))
	if large, err := ReadFile(root, "large"); err != nil || large.Size != 2<<20 {
		t.Fatalf("large = %#v, %v", large, err)
	}
}

func TestReadFileRejectsUnsafeTargets(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "missing"); !errors.Is(err, ErrMissing) {
		t.Fatalf("missing: %v", err)
	}
	if _, err := ReadFile(root, "dir"); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("directory: %v", err)
	}
	if _, err := ReadFile(root, "../escape"); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("escape: %v", err)
	}
	if _, err := ReadFile(root, ".devflow/state.json"); !errors.Is(err, ErrInsideDevflow) {
		t.Fatalf(".devflow: %v", err)
	}
	target := filepath.Join(root, "target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target", filepath.Join(root, "leaf")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "leaf"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("leaf symlink: %v", err)
	}
	if err := os.Symlink("dir", filepath.Join(root, "parent")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "parent/file"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("parent symlink: %v", err)
	}
	if err := os.Symlink("absent", filepath.Join(root, "broken")); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "broken"); !errors.Is(err, ErrSymlink) {
		t.Fatalf("broken symlink: %v", err)
	}
}

func TestReadFileRejectsSpecialFilesBeforeOpening(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "fifo"); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("FIFO: %v", err)
	}
	socket := filepath.Join(root, "socket")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if _, err := ReadFile(root, "socket"); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("socket: %v", err)
	}
	if _, err := ReadFile("/", "dev/null"); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("character device: %v", err)
	}
}

func TestReadFileClassifiesOpenPermissionErrorAsUnreadable(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := openFile
	openFile = func(string) (*os.File, error) { return nil, fs.ErrPermission }
	t.Cleanup(func() { openFile = old })
	if _, err := ReadFile(root, "file"); !errors.Is(err, ErrUnreadable) || errors.Is(err, ErrMissing) {
		t.Fatalf("permission error: %v", err)
	}
}

func TestReadFileDetectsChangeAfterHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := afterHash
	afterHash = func() { _ = os.WriteFile(path, []byte("changed content"), 0o644) }
	t.Cleanup(func() { afterHash = old })
	if _, err := ReadFile(root, "file"); !errors.Is(err, ErrChanged) {
		t.Fatalf("changed: %v", err)
	}
}

func TestReadFileDetectsPathReplacementAfterHash(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := afterHash
	afterHash = func() {
		_ = os.Rename(path, path+".old")
		_ = os.WriteFile(path, []byte("before"), 0o644)
	}
	t.Cleanup(func() { afterHash = old })
	if _, err := ReadFile(root, "file"); !errors.Is(err, ErrChanged) {
		t.Fatalf("replacement: %v", err)
	}
}

func TestReadFileDetectsParentSymlinkReplacementAfterHash(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	other := filepath.Join(root, "other")
	if err := os.Mkdir(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(other, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "file"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other, "file"), []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := afterHash
	afterHash = func() {
		_ = os.Rename(parent, parent+".old")
		_ = os.Symlink("other", parent)
	}
	t.Cleanup(func() { afterHash = old })
	if _, err := ReadFile(root, "parent/file"); !errors.Is(err, ErrChanged) {
		t.Fatalf("parent replacement: %v", err)
	}
}

func TestReadFileAllowsSymlinkProjectRoot(t *testing.T) {
	realRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(realRoot, "file"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "root")
	if err := os.Symlink(realRoot, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(link, "file"); err != nil {
		t.Fatal(err)
	}
}
