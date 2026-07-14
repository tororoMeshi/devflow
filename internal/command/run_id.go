package command

import (
	"crypto/rand"
	"encoding/hex"
)

func newFlowRunID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "run_" + hex.EncodeToString(bytes), nil
}
