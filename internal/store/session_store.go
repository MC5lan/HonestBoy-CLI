package store

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MC5lan/HonestBoy-CLI/internal/core"
	"github.com/gofrs/flock"
)

const schemaVersion = 1

// sessionMaxBytes 是单分片上限；超过后追加到 id-2.jsonl、id-3.jsonl。
var sessionMaxBytes int64 = 50 << 20

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionLocked   = errors.New("session is locked by another process")
	// errTruncatedTail 只用于标记 jsonl 尾行被截断（强杀/写一半），由 Load 决定是否跳过。
	errTruncatedTail = errors.New("truncated jsonl tail")
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
	Ended        bool // 是否写入 session_end；false 表示强杀/崩溃
	CreatedAt    time.Time
	ModifiedAt   time.Time
}

// SessionStore 以 O_APPEND 逐行写入会话，写完 Sync。
type SessionStore struct {
	Paths     Paths
	sessionID string
	part      int // 当前写入分片，从 1 开始
	lock      *flock.Flock
	// RecoveredTail 标记最近一次 Load 是否跳过了被截断的尾行，调用方可据此提示用户。
	RecoveredTail bool
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

func (s *SessionStore) partPath(id string, part int) string {
	if part <= 1 {
		return s.path(id)
	}
	return filepath.Join(s.Paths.Sessions, id+"-"+strconv.Itoa(part)+".jsonl")
}

// parts 返回从 1 开始连续存在的分片路径。
func (s *SessionStore) parts(id string) []string {
	var out []string
	first := s.path(id)
	if _, err := os.Stat(first); err == nil {
		out = append(out, first)
	}
	for part := 2; ; part++ {
		p := s.partPath(id, part)
		if _, err := os.Stat(p); err != nil {
			break
		}
		out = append(out, p)
	}
	return out
}

// isPartFile 判断是否是 id-2.jsonl 这类续片，List/AllReceipts 不应把它当独立会话。
func isPartFile(name string) bool {
	base := strings.TrimSuffix(name, ".jsonl")
	i := strings.LastIndex(base, "-")
	if i <= 0 || i == len(base)-1 {
		return false
	}
	_, err := strconv.Atoi(base[i+1:])
	return err == nil
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
	part := s.part
	if part <= 0 {
		part = 1
	}
	path := s.partPath(id, part)
	// meta 永远留在第一片；事件/回执达到上限后切到下一片。
	if v.Kind != "meta" {
		if info, statErr := os.Stat(path); statErr == nil && info.Size() >= sessionMaxBytes {
			part++
			path = s.partPath(id, part)
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err = f.Write(append(line, '\n')); err != nil {
		return err
	}
	if v.Kind != "meta" {
		s.part = part
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
	s.part = 1
	return s.appendLine(meta.ID, jsonlLine{
		V: schemaVersion, Kind: "meta", ID: meta.ID, Cwd: meta.Cwd, CreatedAt: meta.CreatedAt,
	})
}

// Open 锁定并加载已有会话，供下一回合续接（不重写 meta 行）。
func (s *SessionStore) Open(id string) (*core.Session, error) {
	if err := s.Lock(id); err != nil {
		return nil, err
	}
	sess, err := s.Load(id)
	if err != nil {
		_ = s.Unlock()
		return nil, err
	}
	s.sessionID = id
	return sess, nil
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

// applyLine 解析一条完整 jsonl 行并并入 Session。
// 外层 JSON 损坏标记为 errTruncatedTail，由 Load 判断能否作为尾行跳过。
func (s *SessionStore) applyLine(sess *core.Session, line []byte) error {
	var w jsonlLine
	if err := json.Unmarshal(line, &w); err != nil {
		// 保留原有错误文案，同时挂上标记供 Load 判断能否作为尾行跳过。
		return fmt.Errorf("bad jsonl line: %w (%w)", err, errTruncatedTail)
	}
	if w.V > schemaVersion {
		return fmt.Errorf("schema version %d newer than supported %d", w.V, schemaVersion)
	}
	switch w.Kind {
	case "meta":
		sess.ID, sess.Cwd, sess.CreatedAt = w.ID, w.Cwd, w.CreatedAt
	case "event":
		ev, err := core.UnmarshalEvent(w.Event)
		if err != nil {
			return err
		}
		sess.Events = append(sess.Events, ev)
		if ev.Seq > sess.Seq() {
			sess.SetSeq(ev.Seq)
		}
	case "receipt":
		var r core.Receipt
		if err := json.Unmarshal(w.Receipt, &r); err != nil {
			return err
		}
		sess.Receipts = append(sess.Receipts, r)
	}
	return nil
}

// Load 从 jsonl 全部分片重建 Session（事件按 Type 分派为具体结构）。
// 强杀可能留下半行：只跳过"最后一个分片的最后一行"且仅当外层 JSON 解析失败，
// 其余位置的损坏仍然报错，避免把中途损坏误判成截断。
func (s *SessionStore) Load(id string) (*core.Session, error) {
	parts := s.parts(id)
	if len(parts) == 0 {
		return nil, ErrSessionNotFound
	}
	s.RecoveredTail = false
	sess := &core.Session{ID: id}
	for partIdx, path := range parts {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
		var pending []byte // 暂存上一行，确认后面还有行才解析它
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(strings.TrimSpace(string(line))) == 0 {
				continue
			}
			if pending != nil {
				if err := s.applyLine(sess, pending); err != nil {
					f.Close()
					return nil, err
				}
			}
			pending = append(pending[:0], line...)
		}
		if pending != nil {
			if err := s.applyLine(sess, pending); err != nil {
				// 只有末片的尾行截断可以恢复；中间分片或中间行的损坏必须报错。
				if partIdx == len(parts)-1 && errors.Is(err, errTruncatedTail) {
					s.RecoveredTail = true
				} else {
					f.Close()
					return nil, err
				}
			}
		}
		err = scanner.Err()
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	s.part = len(parts)
	return sess, nil
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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || isPartFile(entry.Name()) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		meta := SessionMeta{ID: id}
		for _, path := range s.parts(id) {
			if info, err := os.Stat(path); err == nil && info.ModTime().After(meta.ModifiedAt) {
				meta.ModifiedAt = info.ModTime()
			}
			f, err := os.Open(path)
			if err != nil {
				continue
			}
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
					if json.Unmarshal(w.Event, &ev) == nil {
						if ev.Type == core.EvtTurnStart && meta.InputSummary == "" {
							meta.InputSummary = truncateRunes(ev.Data.Input, 40)
						}
						if ev.Type == core.EvtSessionEnd {
							meta.Ended = true
						}
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

// Delete 删除一个会话的全部分片及其检查点目录。
func (s *SessionStore) Delete(id string) error {
	_ = os.RemoveAll(filepath.Join(s.Paths.Sessions, id))
	for _, path := range s.parts(id) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
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
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || isPartFile(entry.Name()) {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".jsonl")
		for _, path := range s.parts(id) {
			f, err := os.Open(path)
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
