package testserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// Script 描述按请求次数返回的 chunk 序列。
type Script struct {
	Responses    [][]string
	StatusCodes  []int
	InvalidUTF8  bool
	DisconnectAt int
}

// Server 是脚本化模型服务器，记录收到的请求体供断言。
type Server struct {
	URL      string
	Requests [][]byte
	mu       sync.Mutex
	index    int
	script   Script
	srv      *httptest.Server
}

func New(t *testing.T, script Script) *Server {
	t.Helper()
	s := &Server{script: script}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.Requests = append(s.Requests, body)
		i := s.index
		s.index++
		s.mu.Unlock()

		if i < len(s.script.StatusCodes) && s.script.StatusCodes[i] >= 400 {
			w.WriteHeader(s.script.StatusCodes[i])
			_, _ = fmt.Fprint(w, `{"error":"scripted failure"}`)
			return
		}
		if i >= len(s.script.Responses) {
			http.Error(w, "no scripted response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for j, payload := range s.script.Responses[i] {
			if s.script.DisconnectAt > 0 && j == s.script.DisconnectAt {
				return
			}
			if s.script.InvalidUTF8 {
				// 把占位符换成真正的非法字节，模拟模型吐出乱码（适配器会 SanitizeUTF8）。
				payload = strings.ReplaceAll(payload, "%BAD%", string([]byte{0xff, 0xfe}))
			}
			_, _ = fmt.Fprintf(w, "data: %s\n\n", payload)
			if flusher != nil {
				flusher.Flush()
			}
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		if flusher != nil {
			flusher.Flush()
		}
	}))
	s.URL = s.srv.URL
	t.Cleanup(s.Close)
	return s
}

func (s *Server) Close() {
	if s.srv != nil {
		s.srv.Close()
	}
}

// LastRequest 返回最近一次请求的 JSON body（用于断言上下文装配）。
func (s *Server) LastRequest() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Requests) == 0 {
		return nil
	}
	var out map[string]any
	_ = json.Unmarshal(s.Requests[len(s.Requests)-1], &out)
	return out
}

// RequestCount 返回收到的请求数。
func (s *Server) RequestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}
