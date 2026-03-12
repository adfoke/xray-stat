package logs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"xray-stat/internal/config"
)

const pollInterval = 350 * time.Millisecond

type Entry struct {
	Source string
	Line   string
	Time   time.Time
}

type Follower struct {
	mode           string
	sources        []source
	bootstrapLines int
}

type source struct {
	name string
	path string
}

func NewFollower(mode, errorPath, accessPath string, bootstrapLines int) (*Follower, error) {
	var srcs []source
	switch mode {
	case config.LogModeErrorOnly:
		srcs = appendIfPath(srcs, source{name: "ERR", path: errorPath})
	case config.LogModeAccessOnly:
		srcs = appendIfPath(srcs, source{name: "ACC", path: accessPath})
	case config.LogModeMerge:
		srcs = appendIfPath(srcs, source{name: "ERR", path: errorPath})
		srcs = appendIfPath(srcs, source{name: "ACC", path: accessPath})
	default:
		return nil, fmt.Errorf("未知日志模式: %s", mode)
	}

	if len(srcs) == 0 {
		return nil, errors.New("没有可用日志文件路径")
	}
	return &Follower{
		mode:           mode,
		sources:        srcs,
		bootstrapLines: bootstrapLines,
	}, nil
}

func appendIfPath(s []source, item source) []source {
	if strings.TrimSpace(item.path) == "" {
		return s
	}
	return append(s, item)
}

func (f *Follower) Run(ctx context.Context, out chan<- Entry) {
	var wg sync.WaitGroup

	for _, src := range f.sources {
		lines, err := tailLastLines(src.path, f.bootstrapLines)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			out <- Entry{
				Source: "SYS",
				Line:   fmt.Sprintf("读取历史日志失败 [%s]: %v", src.path, err),
				Time:   time.Now(),
			}
		}
		for _, line := range lines {
			out <- Entry{
				Source: src.name,
				Line:   line,
				Time:   time.Now(),
			}
		}
	}

	for _, src := range f.sources {
		wg.Add(1)
		go func(s source) {
			defer wg.Done()
			f.followOne(ctx, s, out)
		}(src)
	}

	wg.Wait()
}

func (f *Follower) followOne(ctx context.Context, src source, out chan<- Entry) {
	offset, err := fileSize(src.path)
	if err != nil {
		offset = 0
	}
	var rest string
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			chunk, nextOffset, newRest, readErr := readNewLines(src.path, offset, rest)
			if readErr != nil {
				if !errors.Is(readErr, os.ErrNotExist) {
					out <- Entry{
						Source: "SYS",
						Line:   fmt.Sprintf("日志读取错误 [%s]: %v", src.path, readErr),
						Time:   time.Now(),
					}
				}
				continue
			}
			offset = nextOffset
			rest = newRest
			for _, line := range chunk {
				out <- Entry{
					Source: src.name,
					Line:   line,
					Time:   time.Now(),
				}
			}
		}
	}
}

func tailLastLines(path string, count int) ([]string, error) {
	if count <= 0 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := stat.Size()
	if size <= 0 {
		return nil, nil
	}

	const maxRead = int64(1 << 20) // 1MB
	start := size - maxRead
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}

	// 不是从文件头开始读取时，丢弃第一行残片。
	if start > 0 {
		if idx := bytes.IndexByte(data, '\n'); idx >= 0 && idx < len(data)-1 {
			data = data[idx+1:]
		}
	}
	lines := splitLines(string(data))
	if len(lines) <= count {
		return lines, nil
	}
	return lines[len(lines)-count:], nil
}

func splitLines(text string) []string {
	raw := strings.Split(text, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func readNewLines(path string, offset int64, rest string) (lines []string, nextOffset int64, nextRest string, err error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, offset, rest, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, offset, rest, err
	}
	size := stat.Size()
	if size < offset {
		offset = 0
		rest = ""
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, rest, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, offset, rest, err
	}

	nextOffset = offset + int64(len(data))
	if len(data) == 0 {
		return nil, nextOffset, rest, nil
	}

	merged := rest + string(data)
	parts := strings.Split(merged, "\n")
	if len(parts) == 0 {
		return nil, nextOffset, rest, nil
	}

	nextRest = parts[len(parts)-1]
	parts = parts[:len(parts)-1]
	lines = make([]string, 0, len(parts))
	for _, line := range parts {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nextOffset, nextRest, nil
}

func fileSize(path string) (int64, error) {
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return st.Size(), nil
}
