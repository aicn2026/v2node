package sb

import (
	"bytes"
	"os"
)

// nodeLogWriter prefixes every sing-box console line with the panel node tag
// so multi-node hosts can tell which inbound produced the log.
type nodeLogWriter struct {
	tag string
}

func newNodeLogWriter(tag string) *nodeLogWriter {
	return &nodeLogWriter{tag: tag}
}

func (w *nodeLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	prefix := []byte("node[" + w.tag + "] ")
	var out bytes.Buffer
	out.Grow(len(prefix) + len(p) + bytes.Count(p, []byte{'\n'})*len(prefix))
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] != '\n' {
			continue
		}
		line := p[start : i+1]
		if len(bytes.TrimSpace(line)) > 0 {
			out.Write(prefix)
		}
		out.Write(line)
		start = i + 1
	}
	if start < len(p) {
		rest := p[start:]
		if len(bytes.TrimSpace(rest)) > 0 {
			out.Write(prefix)
		}
		out.Write(rest)
	}
	n, err := os.Stderr.Write(out.Bytes())
	if err != nil {
		return 0, err
	}
	_ = n
	return len(p), nil
}
