package executionreport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func CanonicalBytes(report Report) ([]byte, error) {
	if err := Validate(report); err != nil {
		return nil, err
	}
	data, err := json.Marshal(report)
	if err != nil {
		return nil, fmt.Errorf("%w: canonicalize", ErrInvalidReport)
	}
	if len(data) > MaxDocumentBytes {
		return nil, ErrTooLarge
	}
	return data, nil
}

func Digest(report Report) (string, error) {
	data, err := CanonicalBytes(report)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
