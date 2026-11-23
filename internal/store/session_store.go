package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/gofrs/flock"
)

const schemaVersion = 1

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionLocked   = errors.New("session is locked by another process")
)

type jsonlLine struct {
	V         int             `json:"v"`
	Kind      string          `json:"kind"`
	ID        string          `json:"id,omitempty"`
	Cwd       string          `json:"cwd,omitempty"`
	CreatedAt time.Time       `json:"created_at,omitempty"`
	Event     json.RawMessage `json:"event,omitempty"`
	Receipt   json.RawMessage `json:"receipt,omitempty"`
}

// SessionMeta 是列表视图需要的精简信息，不解析整份文件。
type SessionMeta struct {
	ID           string
	Cwd          string
	InputSummary string
	Terminal     string
	CreatedAt    time.Time
	ModifiedAt   time.Time
}

// SessionStore 以 O_APPEND 逐行写入会话，写完 Sync。
type SessionStore struct {
	Paths     Paths
	sessionID string
	lock      *flock.Flock
}

func NewSessionStore(p Paths) *SessionStore { return &SessionStore{Paths: p} }

// Lock 以文件锁独占一个会话；第二个进程只能只读，不能写。
func (s *SessionStore) Lock(id string) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	s.lock = flock.New(s.path(id) + ".lock")
	ok, err := s.lock.TryLock()
	if err != nil {
		return err
	}
	if !ok {
		s.lock = nil
		return ErrSessionLocked
	}
	return nil
}

func (s *SessionStore) Unlock() error {
	if s.lock == nil {
		return nil
	}
	err := s.lock.Unlock()
	s.lock = nil
	return err
}

func (s *SessionStore) path(id string) string {
	return filepath.Join(s.Paths.Sessions, id+".jsonl")
}

func (s *SessionStore) ensureDir() error {
	return os.MkdirAll(s.Paths.Sessions, 0o700)
}

func (s *SessionStore) appendLine(id string, v jsonlLine) error {
	if err := s.ensureDir(); err != nil {
		return err
	}
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.path(id), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

func (s *SessionStore) Create(meta core.Session) error {
	if s.lock != nil {
		_ = s.Unlock()
	}
	if err := s.Lock(meta.ID); err != nil {
		return err
	}
	s.sessionID = meta.ID
	return s.appendLine(meta.ID, jsonlLine{
		V: schemaVersion, Kind: "meta", ID: meta.ID, Cwd: meta.Cwd, CreatedAt: meta.CreatedAt,
	})
}

func (s *SessionStore) SessionID() string { return s.sessionID }

func (s *SessionStore) AppendEvent(e core.Event) error {
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	return s.appendLine(s.sessionID, jsonlLine{V: schemaVersion, Kind: "event", Event: raw})
}

func (s *SessionStore) AppendReceipt(r core.Receipt) error {
	raw, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.appendLine(s.sessionID, jsonlLine{V: schemaVersion, Kind: "receipt", Receipt: raw})
}

// Load 从 jsonl 重建 Session（事件按 Type 分派为具体结构）。
func (s *SessionStore) Load(id string) (*core.Session, error) {
	f, err := os.Open(s.path(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	defer f.Close()
	sess := &core.Session{ID: id}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var w jsonlLine
		if err := json.Unmarshal(line, &w); err != nil {
			return nil, fmt.Errorf("bad jsonl line: %w", err)
		}
		if w.V > schemaVersion {
			return nil, fmt.Errorf("schema version %d newer than supported %d", w.V, schemaVersion)
		}
		switch w.Kind {
		case "meta":
			sess.ID, sess.Cwd, sess.CreatedAt = w.ID, w.Cwd, w.CreatedAt
		case "event":
			ev, err := core.UnmarshalEvent(w.Event)
			if err != nil {
				return nil, err
			}
			sess.Events = append(sess.Events, ev)
			if ev.Seq > sess.Seq() {
				sess.SetSeq(ev.Seq)
			}
		case "receipt":
			var r core.Receipt
			if err := json.Unmarshal(w.Receipt, &r); err != nil {
				return nil, err
			}
			sess.Receipts = append(sess.Receipts, r)
		}
	}
	return sess, scanner.Err()
}

// List 只读元信息行与文件修改时间，按修改时间倒序。
func (s *SessionStore) List() ([]SessionMeta, error) {
	entries, err := os.ReadDir(s.Paths.Sessions)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionMeta
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		meta := SessionMeta{ID: id, ModifiedAt: info.ModTime()}
		f, err := os.Open(s.path(id))
		if err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				var w jsonlLine
				if json.Unmarshal(scanner.Bytes(), &w) != nil {
					continue
				}
				switch w.Kind {
				case "meta":
					meta.Cwd, meta.CreatedAt = w.Cwd, w.CreatedAt
				case "event":
					var ev struct {
						Type core.EventType `json:"type"`
						Data struct {
							Input string `json:"input"`
						} `json:"data"`
					}
					if json.Unmarshal(w.Event, &ev) == nil && ev.Type == core.EvtTurnStart && meta.InputSummary == "" {
						meta.InputSummary = truncateRunes(ev.Data.Input, 40)
					}
				case "receipt":
					var r struct {
						Terminal core.Terminal `json:"terminal"`
					}
					if json.Unmarshal(w.Receipt, &r) == nil && r.Terminal != "" {
						meta.Terminal = string(r.Terminal)
					}
				}
			}
			f.Close()
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModifiedAt.After(out[j].ModifiedAt) })
	return out, nil
}

// Delete 删除一个会话文件及其检查点目录。
func (s *SessionStore) Delete(id string) error {
	_ = os.RemoveAll(filepath.Join(s.Paths.Sessions, id))
	return os.Remove(s.path(id))
}

// DirSize 返回会话目录占用字节数。
func (s *SessionStore) DirSize() (int64, error) {
	var total int64
	entries, err := os.ReadDir(s.Paths.Sessions)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		if e.IsDir() {
			_ = filepath.WalkDir(filepath.Join(s.Paths.Sessions, e.Name()), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				if fi, err := d.Info(); err == nil {
					total += fi.Size()
				}
				return nil
			})
			continue
		}
		total += info.Size()
	}
	return total, nil
}

// AllReceipts 汇总所有会话的 Receipt，供 verify 与 stats 使用。
func (s *SessionStore) AllReceipts() ([]core.Receipt, error) {
	entries, err := os.ReadDir(s.Paths.Sessions)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []core.Receipt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		f, err := os.Open(s.path(strings.TrimSuffix(entry.Name(), ".jsonl")))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		for scanner.Scan() {
			var w jsonlLine
			if json.Unmarshal(scanner.Bytes(), &w) != nil || w.Kind != "receipt" {
				continue
			}
			var r core.Receipt
			if json.Unmarshal(w.Receipt, &r) == nil {
				out = append(out, r)
			}
		}
		f.Close()
	}
	return out, nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
