package domain_test

import (
	"RTTH/internal/domain"
	"RTTH/internal/persist"
	"RTTH/internal/structs"
	"os"
	"testing"
	"time"
)

func newNode(t *testing.T, id int) *domain.Node {
	t.Helper()
	n, err := domain.NewNode(id, 150, t.TempDir())
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	return n
}

func TestNewNode_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "initialization defaults",
			run: func(t *testing.T) {
				n := newNode(t, 1)
				if n.Id != 1 || n.State != "Follower" {
					t.Fatalf("unexpected identity or state: id=%d state=%s", n.Id, n.State)
				}
				if n.Timeout < 150 || n.Timeout >= 300 {
					t.Fatalf("timeout out of range: %d", n.Timeout)
				}
				if len(n.OtherNodes) != 3 {
					t.Fatalf("OtherNodes: want 3 got %d", len(n.OtherNodes))
				}
			},
		},
		{
			name: "loads persisted state",
			run: func(t *testing.T) {
				dir := t.TempDir()
				s, _ := persist.NewStorage(dir, 1)
				_ = s.Save(persist.State{
					CurrentTerm: 4,
					VotedFor:    map[int]int{4: 2},
					Log:         []structs.Transaction{{ID: 1, ClientID: 10, Payload: "A->B 50", Timestamp: 1000, Term: 4}},
				})
				n, err := domain.NewNode(1, 150, dir)
				if err != nil {
					t.Fatalf("NewNode after save: %v", err)
				}
				if n.CurrentTerm != 4 || n.VotedFor[4] != 2 || len(n.Log) != 1 {
					t.Fatalf("persisted state mismatch: term=%d vote=%d log=%d", n.CurrentTerm, n.VotedFor[4], len(n.Log))
				}
			},
		},
		{
			name: "bad data directory returns error",
			run: func(t *testing.T) {
				blockingFile := t.TempDir() + "/notadir"
				_ = os.WriteFile(blockingFile, []byte("x"), 0o644)
				if _, err := domain.NewNode(1, 150, blockingFile+"/subdir"); err == nil {
					t.Fatalf("expected error when dataDir path is blocked by a file")
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}

func TestProcessVoteRequest_TableDriven(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(n *domain.Node)
		req       structs.VoteReq
		wantGrant bool
		wantTerm  int
		wantState string
	}{
		{
			name: "stale term rejected",
			setup: func(n *domain.Node) {
				n.CurrentTerm = 5
			},
			req:       structs.VoteReq{Term: 3, CandidateID: 2, LastLogIndex: 0, LastLogTerm: 0},
			wantGrant: false,
			wantTerm:  5,
			wantState: "Follower",
		},
		{
			name:      "first vote in term is granted",
			setup:     func(n *domain.Node) {},
			req:       structs.VoteReq{Term: 1, CandidateID: 2, LastLogIndex: 0, LastLogTerm: 0},
			wantGrant: true,
			wantTerm:  1,
			wantState: "Follower",
		},
		{
			name: "higher term forces step down",
			setup: func(n *domain.Node) {
				n.State = "Leader"
				n.CurrentTerm = 3
			},
			req:       structs.VoteReq{Term: 7, CandidateID: 3, LastLogIndex: 0, LastLogTerm: 0},
			wantGrant: true,
			wantTerm:  7,
			wantState: "Follower",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			n := newNode(t, 1)
			n.Mu.Lock()
			tt.setup(n)
			n.Mu.Unlock()
			resp, err := n.ProcessVoteRequest(tt.req)
			if err != nil {
				t.Fatalf("ProcessVoteRequest: %v", err)
			}
			if resp.VoteGranted != tt.wantGrant {
				t.Fatalf("VoteGranted: want %v got %v", tt.wantGrant, resp.VoteGranted)
			}
			n.Mu.Lock()
			state := n.State
			n.Mu.Unlock()
			if resp.Term != tt.wantTerm {
				t.Fatalf("Term: want %d got %d", tt.wantTerm, resp.Term)
			}
			if state != tt.wantState {
				t.Fatalf("State: want %s got %s", tt.wantState, state)
			}
		})
	}
}

func TestProcessAppendEntries_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(n *domain.Node)
		req         structs.AppendEntriesReq
		wantSuccess bool
		check       func(t *testing.T, n *domain.Node)
	}{
		{
			name: "stale term rejected",
			setup: func(n *domain.Node) {
				n.CurrentTerm = 5
			},
			req:         structs.AppendEntriesReq{Term: 3, LeaderID: 2},
			wantSuccess: false,
			check:       func(t *testing.T, n *domain.Node) {},
		},
		{
			name:        "heartbeat updates leader state",
			setup:       func(n *domain.Node) { n.State = "Candidate" },
			req:         structs.AppendEntriesReq{Term: 2, LeaderID: 3, Entries: []structs.Transaction{}},
			wantSuccess: true,
			check: func(t *testing.T, n *domain.Node) {
				n.Mu.Lock()
				defer n.Mu.Unlock()
				if n.State != "Follower" || n.LeaderId != 3 || n.CurrentTerm != 2 {
					t.Fatalf("unexpected state after heartbeat: state=%s leader=%d term=%d", n.State, n.LeaderId, n.CurrentTerm)
				}
			},
		},
		{
			name:        "appends entries and advances commit index",
			setup:       func(n *domain.Node) {},
			req:         structs.AppendEntriesReq{Term: 1, LeaderID: 2, Entries: []structs.Transaction{{ClientID: 1, Payload: "2 50", Term: 1}, {ClientID: 2, Payload: "1 10", Term: 1}}, LeaderCommit: 2},
			wantSuccess: true,
			check: func(t *testing.T, n *domain.Node) {
				n.Mu.Lock()
				defer n.Mu.Unlock()
				if len(n.Log) != 2 || n.CommitIndex != 2 || n.LastApplied != 2 {
					t.Fatalf("unexpected replication state: log=%d ci=%d la=%d", len(n.Log), n.CommitIndex, n.LastApplied)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			n := newNode(t, 1)
			n.Mu.Lock()
			tt.setup(n)
			n.Mu.Unlock()
			resp, err := n.ProcessAppendEntries(tt.req)
			if err != nil {
				t.Fatalf("ProcessAppendEntries: %v", err)
			}
			if resp.Success != tt.wantSuccess {
				t.Fatalf("Success: want %v got %v", tt.wantSuccess, resp.Success)
			}
			tt.check(t, n)
		})
	}
}

func TestNodeHelpers_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, n *domain.Node)
	}{
		{
			name: "append transaction stores entry",
			run: func(t *testing.T, n *domain.Node) {
				n.Mu.Lock()
				n.State = "Leader"
				n.CurrentTerm = 1
				n.Mu.Unlock()
				idx, err := n.AppendTransaction(1, "2 100", time.Now().UnixMilli())
				if err != nil {
					t.Fatalf("AppendTransaction: %v", err)
				}
				if idx != 1 {
					t.Fatalf("want index 1 got %d", idx)
				}
			},
		},
		{
			name: "wait for commit timeout false",
			run: func(t *testing.T, n *domain.Node) {
				if n.WaitForCommit(1, 100*time.Millisecond) {
					t.Fatalf("expected false when commit never advances")
				}
			},
		},
		{
			name: "wait for commit true after update",
			run: func(t *testing.T, n *domain.Node) {
				go func() {
					time.Sleep(50 * time.Millisecond)
					n.Mu.Lock()
					n.CommitIndex = 1
					n.Mu.Unlock()
				}()
				if !n.WaitForCommit(1, 500*time.Millisecond) {
					t.Fatalf("expected true when commit advances")
				}
			},
		},
		{
			name: "get balance includes pending",
			run: func(t *testing.T, n *domain.Node) {
				n.Mu.Lock()
				n.Log = append(n.Log, structs.Transaction{ClientID: 1, Payload: "2 100", Term: 1})
				n.Mu.Unlock()
				_, pendingSender := n.GetBalance(1)
				_, pendingReceiver := n.GetBalance(2)
				if pendingSender != -100 || pendingReceiver != 100 {
					t.Fatalf("unexpected pending balances sender=%d receiver=%d", pendingSender, pendingReceiver)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			n := newNode(t, 1)
			tt.run(t, n)
		})
	}
}
