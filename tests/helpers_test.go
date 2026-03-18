package test

import (
	"RTTH/internal/domain"
	"path/filepath"
	"testing"
)

func newIsolatedNode(t *testing.T, id int, timeout int) *domain.Node {
	t.Helper()
	t.Setenv("RAFT_DATA_DIR", filepath.Join(t.TempDir(), "raft-state"))
	return domain.NewNode(id, timeout)
}
