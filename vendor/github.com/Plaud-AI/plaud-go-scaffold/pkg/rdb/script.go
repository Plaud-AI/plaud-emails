package rdb

import (
	"crypto/sha1"
	"encoding/hex"
	"io"
)

type Script struct {
	src  string
	hash string
}

// GetSha1 获取脚本Sha1
func (s *Script) GetSha1() string {
	return s.hash
}

// NewScript returns a new script object.
func NewScript(src string) *Script {
	h := sha1.New()
	io.WriteString(h, src) // nolint: errcheck
	return &Script{src, hex.EncodeToString(h.Sum(nil))}
}
