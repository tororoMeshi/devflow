package automationruntime

import (
	"bytes"
	"testing"
)

func TestLimitedCaptureBoundaries(t *testing.T) {
	for _, size := range []int{8, 9} {
		capture := newLimitedCapture(8, nil)
		input := bytes.Repeat([]byte("x"), size)
		if _, err := capture.Write(input); err != nil {
			t.Fatal(err)
		}
		got, overflow := capture.snapshot()
		if len(got) != 8 || overflow != (size == 9) {
			t.Fatalf("size %d: len=%d overflow=%t", size, len(got), overflow)
		}
	}
}

func TestTailBufferKeepsTail(t *testing.T) {
	tail := newTailBuffer(4)
	_, _ = tail.Write([]byte("abc"))
	_, _ = tail.Write([]byte("def"))
	got, truncated := tail.snapshot()
	if string(got) != "cdef" || !truncated {
		t.Fatalf("got %q truncated=%t", got, truncated)
	}
}

func TestConfiguredCaptureBoundaries(t *testing.T) {
	for _, limit := range []int{MaxWorkPackageBytes, MaxExecutorStdoutBytes} {
		for _, size := range []int{limit, limit + 1} {
			capture := newLimitedCapture(limit, nil)
			_, _ = capture.Write(bytes.Repeat([]byte("x"), size))
			got, overflow := capture.snapshot()
			if len(got) != limit || overflow != (size > limit) {
				t.Fatalf("limit=%d size=%d len=%d overflow=%t", limit, size, len(got), overflow)
			}
		}
	}
}

func TestTailBufferBoundariesAndChunks(t *testing.T) {
	tests := []struct {
		name      string
		chunks    [][]byte
		want      string
		truncated bool
	}{
		{"empty write", [][]byte{nil}, "", false},
		{"below limit", [][]byte{[]byte("abc")}, "abc", false},
		{"exact limit", [][]byte{[]byte("abcd")}, "abcd", false},
		{"over limit", [][]byte{[]byte("abcde")}, "bcde", true},
		{"multiple chunks exact", [][]byte{[]byte("ab"), []byte("cd")}, "abcd", false},
		{"multiple chunks overflow", [][]byte{[]byte("ab"), []byte("cde")}, "bcde", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tail := newTailBuffer(4)
			for _, chunk := range tt.chunks {
				_, _ = tail.Write(chunk)
			}
			got, truncated := tail.snapshot()
			if string(got) != tt.want || truncated != tt.truncated {
				t.Fatalf("got=%q truncated=%t want=%q,%t", got, truncated, tt.want, tt.truncated)
			}
		})
	}
}
