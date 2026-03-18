package test

import (
	"RTTH/internal/domain"
	"RTTH/internal/structs"
	"os"
	"path/filepath"
	"testing"
)

func TestNode_PersistsVoteStateAcrossRestart(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "raft-state")
	if err := os.Setenv("RAFT_DATA_DIR", dataDir); err != nil {
		t.Fatalf("failed to set RAFT_DATA_DIR: %v", err)
	}
	defer os.Unsetenv("RAFT_DATA_DIR")

	n1 := domain.NewNode(1, 150)
	resp := n1.ProcessVoteRequest(structs.VoteReq{Term: 7, CandidateID: 2})
	if !resp.VoteGranted {
		t.Fatalf("expected first vote to be granted")
	}

	n2 := domain.NewNode(1, 150)
	if n2.CurrentTerm != 7 {
		t.Fatalf("expected restored term 7, got %d", n2.CurrentTerm)
	}
	if n2.VotedFor != 2 {
		t.Fatalf("expected restored votedFor 2, got %d", n2.VotedFor)
	}

	resp2 := n2.ProcessVoteRequest(structs.VoteReq{Term: 7, CandidateID: 3})
	if resp2.VoteGranted {
		t.Fatalf("expected second vote in same restored term to be rejected")
	}
}

func TestNode_PersistsHigherTermFromHeartbeat(t *testing.T) {
	baseDir := t.TempDir()
	dataDir := filepath.Join(baseDir, "raft-state")
	if err := os.Setenv("RAFT_DATA_DIR", dataDir); err != nil {
		t.Fatalf("failed to set RAFT_DATA_DIR: %v", err)
	}
	defer os.Unsetenv("RAFT_DATA_DIR")

	n1 := domain.NewNode(2, 150)
	ok := n1.ProcessHeartbeat(structs.HeartBeat{LeaderID: 1, Term: 9, Heartbeat: '@', Timestamp: 1})
	if !ok {
		t.Fatalf("expected heartbeat with higher term to be accepted")
	}

	n2 := domain.NewNode(2, 150)
	if n2.CurrentTerm != 9 {
		t.Fatalf("expected restored term 9, got %d", n2.CurrentTerm)
	}
	if n2.VotedFor != 0 {
		t.Fatalf("expected restored votedFor 0, got %d", n2.VotedFor)
	}
}
