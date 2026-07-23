package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/8noki8/devflow/internal/pathcheck"
)

var (
	ErrMissing        = errors.New("artifact file missing")
	ErrNotRegular     = errors.New("artifact is not a regular file")
	ErrSymlink        = errors.New("artifact path contains a symlink")
	ErrOutsideProject = errors.New("artifact is outside project")
	ErrInsideDevflow  = errors.New("artifact is inside .devflow")
	ErrUnreadable     = errors.New("artifact is unreadable")
	ErrChanged        = errors.New("artifact changed while hashing")
	ErrInvalidPath    = errors.New("invalid artifact path")
)

type FileEvidence struct {
	Digest string
	Size   int64
}

var afterHash = func() {}
var openFile = os.Open

func ReadFile(projectRoot, relativePath string) (FileEvidence, error) {
	if err := pathcheck.ValidateArtifactPath(relativePath); err != nil {
		return FileEvidence{}, ErrInvalidPath
	}
	if strings.Split(relativePath, "/")[0] == ".devflow" {
		return FileEvidence{}, ErrInsideDevflow
	}
	root, err := filepath.EvalSymlinks(projectRoot)
	if err != nil {
		return FileEvidence{}, fmt.Errorf("%w: project root", ErrUnreadable)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return FileEvidence{}, ErrOutsideProject
	}
	current := root
	segments := strings.Split(relativePath, "/")
	componentPaths := make([]string, 0, len(segments)+1)
	componentInfos := make([]os.FileInfo, 0, len(segments)+1)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return FileEvidence{}, fmt.Errorf("%w: project root", ErrUnreadable)
	}
	componentPaths = append(componentPaths, root)
	componentInfos = append(componentInfos, rootInfo)
	for i, segment := range segments {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				return FileEvidence{}, ErrMissing
			}
			return FileEvidence{}, fmt.Errorf("%w: %v", ErrUnreadable, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return FileEvidence{}, ErrSymlink
		}
		if i < len(segments)-1 && !info.IsDir() {
			return FileEvidence{}, ErrNotRegular
		}
		if i == len(segments)-1 && !info.Mode().IsRegular() {
			return FileEvidence{}, ErrNotRegular
		}
		componentPaths = append(componentPaths, current)
		componentInfos = append(componentInfos, info)
	}
	rel, err := filepath.Rel(root, current)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return FileEvidence{}, ErrOutsideProject
	}
	f, err := openFile(current)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileEvidence{}, ErrMissing
		}
		return FileEvidence{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return FileEvidence{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	if !before.Mode().IsRegular() {
		return FileEvidence{}, ErrNotRegular
	}
	hash := sha256.New()
	size, err := io.Copy(hash, f)
	if err != nil {
		return FileEvidence{}, fmt.Errorf("%w: %v", ErrUnreadable, err)
	}
	afterHash()
	after, err := f.Stat()
	if err != nil {
		return FileEvidence{}, ErrChanged
	}
	var pathInfo os.FileInfo
	for i, path := range componentPaths {
		info, statErr := os.Lstat(path)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(componentInfos[i], info) {
			return FileEvidence{}, ErrChanged
		}
		if i < len(componentPaths)-1 && !info.IsDir() {
			return FileEvidence{}, ErrChanged
		}
		if i == len(componentPaths)-1 {
			pathInfo = info
		}
	}
	pathStat, err := os.Stat(current)
	if err != nil {
		return FileEvidence{}, ErrChanged
	}
	if pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() ||
		!pathStat.Mode().IsRegular() ||
		!os.SameFile(before, after) || !os.SameFile(after, pathInfo) || !os.SameFile(pathInfo, pathStat) ||
		before.Size() != after.Size() || after.Size() != pathInfo.Size() || after.Size() != size ||
		pathInfo.Size() != pathStat.Size() ||
		!before.ModTime().Equal(after.ModTime()) || !after.ModTime().Equal(pathInfo.ModTime()) ||
		!pathInfo.ModTime().Equal(pathStat.ModTime()) {
		return FileEvidence{}, ErrChanged
	}
	return FileEvidence{Digest: "sha256:" + hex.EncodeToString(hash.Sum(nil)), Size: size}, nil
}
