package test

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	testDataDir, err := os.MkdirTemp("", "rtth-raft-test-")
	if err != nil {
		panic(err)
	}

	if err := os.Setenv("RAFT_DATA_DIR", testDataDir); err != nil {
		panic(err)
	}

	code := m.Run()
	_ = os.Unsetenv("RAFT_DATA_DIR")
	_ = os.RemoveAll(testDataDir)
	os.Exit(code)
}
