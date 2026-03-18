package test

import (
	"testing"
)

func TestNewNode_Initialization(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)

	if node.Id != 1 {
		t.Errorf("expected Node ID 1, got %d", node.Id)
	}
	if node.State != "Follower" {
		t.Errorf("expected initial state 'Follower', got '%s'", node.State)
	}
	if node.Timeout != 150 {
		t.Errorf("expected timeout 150, got %d", node.Timeout)
	}
	if node.CurrentTerm != 0 {
		t.Errorf("expected initial term 0, got %d", node.CurrentTerm)
	}
	if len(node.OtherNodes) != 3 {
		t.Errorf("expected 3 other nodes seeded initially, got %d", len(node.OtherNodes))
	}
}
