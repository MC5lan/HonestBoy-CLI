package llm

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

// SSE 单行上限：超了报错而不是 OOM。
const maxSSELine = 1 << 20

var (
	ErrSSELineTooLong = errors.New("SSE line exceeds 1MB")
	ErrSSECallback    = errors.New("SSE callback failed")
)

// ParseSSE 解析 SSE 流。
// 规则：\n 与 \r\n 都吃；空行 = 事件分隔；data: 后可有零或一个空格；
// 多行 data: 用 \n 拼接；冒号开头是注释，丢弃；超长行报错。
// 任何畸形输入都不得 panic 或无限循环。
func ParseSSE(r io.Reader, fn func(event, data []byte) error) error {
	br := bufio.NewReader(r)
	var dataLines [][]byte
	var event []byte

	dispatch := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := bytes.Join(dataLines, []byte("\n"))
		if err := fn(event, data); err != nil {
			return errors.Join(ErrSSECallback, err)
		}
		dataLines = nil
		event = nil
		return nil
	}

	for {
		line, err := readSSELine(br)
		if len(line) > 0 {
			switch {
			case line[0] == ':':
				// 心跳注释，丢弃。
			case bytes.HasPrefix(line, []byte("event:")):
				event = bytes.TrimSpace(line[len("event:"):])
			case bytes.HasPrefix(line, []byte("data:")):
				v := line[len("data:"):]
				if len(v) > 0 && v[0] == ' ' {
					v = v[1:] // 只吃掉一个可选空格
				}
				dataLines = append(dataLines, append([]byte(nil), v...))
			default:
				// id / retry 等字段暂不处理。
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return dispatch()
			}
			return err
		}
		if len(line) == 0 {
			if err := dispatch(); err != nil {
				return err
			}
		}
	}
}

// readSSELine 读取一行并去掉 \r，带 1MB 上限。
// EOF 时若已有部分内容，连同内容一起返回，由调用方处理后再收尾。
func readSSELine(br *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		part, err := br.ReadSlice('\n')
		buf = append(buf, part...)
		if len(buf) > maxSSELine {
			return nil, ErrSSELineTooLong
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if len(buf) > 0 && buf[len(buf)-1] == '\n' {
			buf = buf[:len(buf)-1]
		}
		if len(buf) > 0 && buf[len(buf)-1] == '\r' {
			buf = buf[:len(buf)-1]
		}
		return buf, err
	}
}
