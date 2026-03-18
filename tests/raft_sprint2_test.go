package test

import (
	"RTTH/internal/structs"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newPeerServer(t *testing.T, voteFn func(structs.VoteReq) structs.VoteResp) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/requestvote", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var req structs.VoteReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(voteFn(req))
	})
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux)
}

func TestProcessVoteRequest_OneVotePerTerm(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)

	resp1 := node.ProcessVoteRequest(structs.VoteReq{Term: 3, CandidateID: 2})
	if !resp1.VoteGranted {
		t.Fatalf("expected first vote in term to be granted")
	}
	if resp1.Term != 3 {
		t.Fatalf("expected response term 3, got %d", resp1.Term)
	}

	resp2 := node.ProcessVoteRequest(structs.VoteReq{Term: 3, CandidateID: 3})
	if resp2.VoteGranted {
		t.Fatalf("expected second vote in same term to be rejected")
	}

	if node.VotedFor != 2 {
		t.Fatalf("expected node to remain voted for candidate 2, got %d", node.VotedFor)
	}
}

func TestProcessHeartbeat_HigherTermForcesFollower(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	node.State = "Leader"
	node.CurrentTerm = 2
	node.LeaderId = 1

	ok := node.ProcessHeartbeat(structs.HeartBeat{LeaderID: 2, Term: 3, Heartbeat: '@', Timestamp: time.Now().UnixMilli()})
	if !ok {
		t.Fatalf("expected heartbeat with higher term to be accepted")
	}
	if node.State != "Follower" {
		t.Fatalf("expected node state to become Follower, got %s", node.State)
	}
	if node.CurrentTerm != 3 {
		t.Fatalf("expected term to become 3, got %d", node.CurrentTerm)
	}
	if node.LeaderId != 2 {
		t.Fatalf("expected leader id 2, got %d", node.LeaderId)
	}
	if node.LastLeaderTimeStamp <= time.Now().UnixMilli() {
		t.Fatalf("expected election timer deadline to be reset to future time")
	}
}

func TestProcessHeartbeat_StaleTermRejected(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	node.State = "Follower"
	node.CurrentTerm = 4
	node.LeaderId = 1

	ok := node.ProcessHeartbeat(structs.HeartBeat{LeaderID: 2, Term: 3, Heartbeat: '@', Timestamp: time.Now().UnixMilli()})
	if ok {
		t.Fatalf("expected stale heartbeat to be rejected")
	}
	if node.CurrentTerm != 4 {
		t.Fatalf("expected term to remain 4, got %d", node.CurrentTerm)
	}
	if node.LeaderId != 1 {
		t.Fatalf("expected leader id to remain 1, got %d", node.LeaderId)
	}
}

func TestStartElection_BecomesLeaderOnMajority(t *testing.T) {
	s2 := newPeerServer(t, func(req structs.VoteReq) structs.VoteResp {
		return structs.VoteResp{Term: req.Term, VoteGranted: true}
	})
	defer s2.Close()

	s3 := newPeerServer(t, func(req structs.VoteReq) structs.VoteResp {
		return structs.VoteResp{Term: req.Term, VoteGranted: true}
	})
	defer s3.Close()

	node := newIsolatedNode(t, 1, 150)
	node.OtherNodes = map[int]string{
		2: s2.URL,
		3: s3.URL,
	}
	node.State = "Follower"
	node.CurrentTerm = 0

	node.StartElection()

	if node.State != "Leader" {
		t.Fatalf("expected node to become Leader, got %s", node.State)
	}
	if node.LeaderId != 1 {
		t.Fatalf("expected leader id 1, got %d", node.LeaderId)
	}
	if node.CurrentTerm != 1 {
		t.Fatalf("expected current term 1, got %d", node.CurrentTerm)
	}
}

func TestStartElection_StepsDownOnHigherTermVoteResponse(t *testing.T) {
	s2 := newPeerServer(t, func(req structs.VoteReq) structs.VoteResp {
		return structs.VoteResp{Term: req.Term + 1, VoteGranted: false}
	})
	defer s2.Close()

	s3 := newPeerServer(t, func(req structs.VoteReq) structs.VoteResp {
		return structs.VoteResp{Term: req.Term, VoteGranted: true}
	})
	defer s3.Close()

	node := newIsolatedNode(t, 1, 150)
	node.OtherNodes = map[int]string{
		2: s2.URL,
		3: s3.URL,
	}
	node.State = "Follower"
	node.CurrentTerm = 0

	node.StartElection()

	if node.State != "Follower" {
		t.Fatalf("expected node to step down to Follower, got %s", node.State)
	}
	if node.CurrentTerm != 2 {
		t.Fatalf("expected term to move to 2 after higher-term response, got %d", node.CurrentTerm)
	}
}
