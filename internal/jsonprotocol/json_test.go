package jsonprotocol

import (
	"errors"
	"testing"
)

func TestValidateKeysAndTrailing(t *testing.T) {
	tests := []struct {
		name string
		data string
		err  error
	}{
		{"object", `{"a":1,"nested":{"a":2},"array":[{"a":3},{"a":4}]}`, nil},
		{"array root", `[{"a":1},{"a":2}]`, nil},
		{"scalar root", `"value"`, nil},
		{"top-level duplicate", `{"a":1,"a":2}`, ErrDuplicateKey},
		{"escaped duplicate", `{"summary":"a","\u0073ummary":"b"}`, ErrDuplicateKey},
		{"nested duplicate", `{"a":{"b":1,"b":2}}`, ErrDuplicateKey},
		{"array object duplicate", `[{"a":1},{"b":1,"b":2}]`, ErrDuplicateKey},
		{"second value", `{"a":1} {"b":2}`, ErrTrailingJSON},
		{"trailing scalar", `{"a":1} true`, ErrTrailingJSON},
		{"malformed", `{"a":[}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKeysAndTrailing([]byte(tt.data))
			if tt.name == "malformed" {
				if err == nil || errors.Is(err, ErrDuplicateKey) || errors.Is(err, ErrTrailingJSON) {
					t.Fatalf("malformed error=%v", err)
				}
				return
			}
			if !errors.Is(err, tt.err) {
				t.Fatalf("error=%v want=%v", err, tt.err)
			}
		})
	}
}
