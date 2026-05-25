package logbuffer

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
)

const defaultLimit = 600

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

type Buffer struct {
	mu      sync.RWMutex
	limit   int
	lines   []string
	partial string
}

var defaultBuffer = New(defaultLimit)

func Default() *Buffer {
	return defaultBuffer
}

func New(limit int) *Buffer {
	if limit < 1 {
		limit = defaultLimit
	}
	return &Buffer{limit: limit}
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	text := b.partial + string(p)
	parts := strings.Split(text, "\n")
	b.partial = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		b.appendLine(line)
	}
	return len(p), nil
}

func (b *Buffer) Lines(limit int) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	lines := b.lines
	if b.partial != "" {
		lines = append(append([]string{}, b.lines...), cleanLine(b.partial))
	}
	if limit < 1 || limit > len(lines) {
		limit = len(lines)
	}
	out := make([]string, limit)
	copy(out, lines[len(lines)-limit:])
	return out
}

func (b *Buffer) appendLine(line string) {
	line = cleanLine(line)
	if strings.TrimSpace(line) == "" {
		return
	}
	b.lines = append(b.lines, line)
	if len(b.lines) > b.limit {
		copy(b.lines, b.lines[len(b.lines)-b.limit:])
		b.lines = b.lines[:b.limit]
	}
}

func cleanLine(line string) string {
	line = ansiPattern.ReplaceAllString(line, "")
	line = strings.TrimRight(line, "\r")
	return string(bytes.TrimRight([]byte(line), "\x00"))
}
