package kubeapply

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

// StatusWriter wraps an io.Writer so that it always has a footer at
// the bottom displaying several status messages.
type StatusWriter struct {
	inner    io.Writer
	statuses map[string]string

	mu sync.Mutex

	stateLastLine      string
	stateStatusLineCnt int
}

// NewStatusWriter creates a new StatusWriter.
func NewStatusWriter(inner io.Writer) *StatusWriter {
	return &StatusWriter{
		inner:    inner,
		statuses: make(map[string]string),
	}
}

func (sw *StatusWriter) writeStatus() error {
	if len(sw.statuses) == 0 {
		return nil
	}

	keyLen := 0
	valLen := 0
	keys := make([]string, 0, len(sw.statuses))
	for key, val := range sw.statuses {
		if len(key) > keyLen { // XXX: Unicode
			keyLen = len(key)
		}
		if len(val) > valLen { // XXX: Unicode
			valLen = len(val)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	div := make([]byte, keyLen+2+valLen)
	for i := range div {
		div[i] = '-'
	}

	sw.stateStatusLineCnt = 0
	if _, err := fmt.Fprintf(sw.inner, "%s\x1B[0K\n", div); err != nil {
		return err
	}
	sw.stateStatusLineCnt++
	for _, key := range keys {
		if _, err := fmt.Fprintf(sw.inner, "%-[1]*[2]s: %s\x1B[0K\n", keyLen, key, sw.statuses[key]); err != nil {
			return err
		}
		sw.stateStatusLineCnt++
	}

	return nil
}

func (sw *StatusWriter) Write(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	linecnt := sw.stateStatusLineCnt
	if sw.stateLastLine != "" {
		linecnt++
	}
	if linecnt > 0 {
		if _, err := fmt.Fprintf(sw.inner, "\x1B[%dA%s", linecnt, sw.stateLastLine); err != nil {
			return 0, err
		}
	}

	nbytes := 0
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		n, err := io.WriteString(sw.inner, line)
		nbytes += n
		if err != nil {
			return nbytes, err
		}
		sw.stateLastLine = line

		if _, err := io.WriteString(sw.inner, "\x1B[0K"); err != nil {
			return nbytes, err
		}

		if i < len(lines)-1 {
			// normal line
			n, err = io.WriteString(sw.inner, "\n")
			nbytes += n
			if err != nil {
				return nbytes, err
			}
		} else {
			// last line
			if line != "" {
				if _, err := io.WriteString(sw.inner, "\n"); err != nil {
					return nbytes, err
				}
			}
		}
	}

	if err := sw.writeStatus(); err != nil {
		return nbytes, err
	}

	return nbytes, nil
}

func (sw *StatusWriter) SetStatus(key, val string) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	if oldval, oldvalOK := sw.statuses[key]; oldvalOK && oldval == val {
		return nil
	}

	sw.statuses[key] = val

	if sw.stateStatusLineCnt > 0 {
		if _, err := fmt.Fprintf(sw.inner, "\x1B[%dA", sw.stateStatusLineCnt); err != nil {
			return err
		}
	}
	if err := sw.writeStatus(); err != nil {
		return err
	}

	return nil
}