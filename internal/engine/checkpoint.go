package engine

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Checkpoint 是回合开始时的文件快照（对标 Claude Code /rewind 与 Aider /undo）。
type Checkpoint struct {
	ID        string                    `json:"id"`
	TurnID    string                    `json:"turn_id"`
	Prompt    string                    `json:"prompt"`
	CreatedAt time.Time                 `json:"created_at"`
	Files     map[string]checkpointFile `json:"files"`
}

type checkpointFile struct {
	Data   string `json:"data"` // base64；为空表示文件原本不存在
	Exists bool   `json:"exists"`
	Mode   uint32 `json:"mode"`
}

var checkpointIgnored = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "bin": true,
	"dist": true, ".idea": true, "__pycache__": true, ".hb-crash": true,
}

// CreateCheckpoint 扫描工作区（跳过忽略目录与 .lock/.jsonl），记录每个文件的字节快照。
func CreateCheckpoint(workdir, sessionDir, turnID, prompt string) (*Checkpoint, error) {
	cp := &Checkpoint{
		ID:     "cp_" + shortID(turnID+time.Now().UTC().Format(time.RFC3339Nano)),
		TurnID: turnID, Prompt: prompt, CreatedAt: time.Now().UTC(),
		Files: make(map[string]checkpointFile),
	}
	err := filepath.WalkDir(workdir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(workdir, path)
		if rerr != nil || rel == "." {
			return nil
		}
		if d.IsDir() {
			if checkpointIgnored[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(rel, ".lock") || strings.HasSuffix(rel, ".jsonl") {
			return nil
		}
		info, _ := d.Info()
		if info != nil && info.Size() > 2<<20 {
			return nil // 大文件不进快照
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		mode := uint32(0o644)
		if info != nil {
			mode = uint32(info.Mode().Perm())
		}
		cp.Files[filepath.ToSlash(rel)] = checkpointFile{
			Data: base64.StdEncoding.EncodeToString(data), Exists: true, Mode: mode,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := SaveCheckpoint(sessionDir, cp); err != nil {
		return nil, err
	}
	return cp, nil
}

func checkpointDir(sessionDir string) string { return filepath.Join(sessionDir, "checkpoints") }

func SaveCheckpoint(sessionDir string, cp *Checkpoint) error {
	dir := checkpointDir(sessionDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cp.ID+".json"), data, 0o600)
}

func ListCheckpoints(sessionDir string) ([]Checkpoint, error) {
	entries, err := os.ReadDir(checkpointDir(sessionDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Checkpoint
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(checkpointDir(sessionDir), e.Name()))
		if err != nil {
			continue
		}
		var cp Checkpoint
		if json.Unmarshal(data, &cp) == nil {
			out = append(out, cp)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// RestoreCheckpoint 把工作区恢复到快照状态：覆盖已有文件，删除快照中不存在的新增文件。
func RestoreCheckpoint(workdir string, cp *Checkpoint) (int, error) {
	if cp == nil {
		return 0, fmt.Errorf("checkpoint is nil")
	}
	restored := 0
	for rel, f := range cp.Files {
		path := filepath.Join(workdir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(f.Data)
		if err != nil {
			continue
		}
		mode := os.FileMode(f.Mode)
		if mode == 0 {
			mode = 0o644
		}
		if err := os.WriteFile(path, data, mode); err != nil {
			return restored, err
		}
		restored++
	}
	// 删除快照之后新增的文件（只在快照根内、且不在忽略目录）。
	_ = filepath.WalkDir(workdir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, rerr := filepath.Rel(workdir, path)
		if rerr != nil {
			return nil
		}
		if _, ok := cp.Files[filepath.ToSlash(rel)]; !ok {
			if !checkpointIgnored[d.Name()] && !strings.HasSuffix(rel, ".lock") && !strings.HasSuffix(rel, ".jsonl") {
				_ = os.Remove(path)
			}
		}
		return nil
	})
	return restored, nil
}
