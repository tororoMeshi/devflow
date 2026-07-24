package automationruntime

import (
	"io"
	"sync"
)

type limitedCapture struct {
	mu       sync.Mutex
	data     []byte
	limit    int
	overflow bool
	onLimit  func()
}

func newLimitedCapture(limit int, onLimit func()) *limitedCapture {
	return &limitedCapture{limit: limit, onLimit: onLimit}
}

func (w *limitedCapture) Write(p []byte) (int, error) {
	w.mu.Lock()
	notify := false
	if !w.overflow {
		remaining := w.limit - len(w.data)
		if remaining >= len(p) {
			w.data = append(w.data, p...)
		} else {
			if remaining > 0 {
				w.data = append(w.data, p[:remaining]...)
			}
			w.overflow = true
			notify = w.onLimit != nil
		}
	}
	w.mu.Unlock()
	if notify {
		w.onLimit()
	}
	return len(p), nil
}

func (w *limitedCapture) snapshot() ([]byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.data...), w.overflow
}

type tailBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func newTailBuffer(limit int) *tailBuffer { return &tailBuffer{limit: limit} }

func (w *tailBuffer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) >= w.limit {
		hadData := len(w.data) > 0
		w.data = append(w.data[:0], p[len(p)-w.limit:]...)
		if len(p) > w.limit || hadData {
			w.truncated = true
		}
		return len(p), nil
	}
	if len(w.data)+len(p) > w.limit {
		drop := len(w.data) + len(p) - w.limit
		copy(w.data, w.data[drop:])
		w.data = w.data[:len(w.data)-drop]
		w.truncated = true
	}
	w.data = append(w.data, p...)
	return len(p), nil
}

func (w *tailBuffer) snapshot() ([]byte, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]byte(nil), w.data...), w.truncated
}

func copyAll(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}
