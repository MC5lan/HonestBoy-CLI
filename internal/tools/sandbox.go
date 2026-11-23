package tools

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var shortNameRe = regexp.MustCompile(`~[0-9]`)

// Sandbox 用 os.Root 在内核调用层拒绝越界。
// 禁止出现裸 os.Open：所有文件访问都经由此处。
type Sandbox struct {
	root    *os.Root
	workdir string
}

func Open(dir string) (*Sandbox, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, errors.New("workdir is not a directory")
	}
	root, err := os.OpenRoot(abs)
	if err != nil {
		return nil, err
	}
	return &Sandbox{root: root, workdir: abs}, nil
}

func (s *Sandbox) Close() error { return s.root.Close() }

func (s *Sandbox) Workdir() string { return s.workdir }

// Rel 把模型给的路径（绝对或相对）收敛到根内；根外路径直接报错。
func (s *Sandbox) Rel(name string) (string, error) {
	if name == "" || strings.ContainsRune(name, 0) {
		return "", ErrOutsideSandbox
	}
	// 编码绕过与盘符相对路径一律拒绝，不做“猜它想指哪个”。
	lower := strings.ToLower(name)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") ||
		(filepath.VolumeName(name) != "" && !filepath.IsAbs(name)) {
		return "", ErrOutsideSandbox
	}
	var rel string
	if filepath.IsAbs(name) {
		r, err := filepath.Rel(s.workdir, name)
		if err != nil {
			return "", ErrOutsideSandbox
		}
		rel = r
	} else {
		if shortNameRe.MatchString(name) {
			return "", ErrOutsideSandbox
		}
		rel = filepath.Clean(name)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", ErrOutsideSandbox
	}
	return rel, nil
}

func (s *Sandbox) OpenFile(name string) (*os.File, error) {
	rel, err := s.Rel(name)
	if err != nil {
		return nil, err
	}
	return s.root.Open(rel)
}

func (s *Sandbox) Create(name string) (*os.File, error) {
	rel, err := s.Rel(name)
	if err != nil {
		return nil, err
	}
	return s.root.Create(rel)
}

func (s *Sandbox) OpenFileRW(name string) (*os.File, error) {
	rel, err := s.Rel(name)
	if err != nil {
		return nil, err
	}
	return s.root.OpenFile(rel, os.O_RDWR, 0)
}

func (s *Sandbox) Stat(name string) (os.FileInfo, error) {
	rel, err := s.Rel(name)
	if err != nil {
		return nil, err
	}
	return s.root.Stat(rel)
}

func (s *Sandbox) Remove(name string) error {
	rel, err := s.Rel(name)
	if err != nil {
		return err
	}
	return s.root.Remove(rel)
}

func (s *Sandbox) Rename(oldName, newName string) error {
	oldRel, err := s.Rel(oldName)
	if err != nil {
		return err
	}
	newRel, err := s.Rel(newName)
	if err != nil {
		return err
	}
	return s.root.Rename(oldRel, newRel)
}

func (s *Sandbox) ReadFile(name string) ([]byte, error) {
	f, err := s.OpenFile(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxReadBytes+1))
}
