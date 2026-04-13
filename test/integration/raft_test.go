package integration

import (
	"RTTH/internal/domain"
	"RTTH/internal/handlers"
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type testCluster struct {
	nodes   []*domain.Node
	servers []*httptest.Server
}

func newCluster(t *testing.T) *testCluster {
	t.Helper()
	gin.SetMode(gin.TestMode)

	c := &testCluster{
		nodes:   make([]*domain.Node, 3),
		servers: make([]*httptest.Server, 3),
	}

	for i := 0; i < 3; i++ {
		node, err := domain.NewNode(i+1, 300, t.TempDir())
		if err != nil {
			t.Fatalf("NewNode %d: %v", i+1, err)
		}
		c.nodes[i] = node
	}

	for i, node := range c.nodes {
		h := handlers.NewHandler(store.NewMemoryStore(), node)
		r := gin.New()
		r.POST("/appendentries", h.HandleAppendEntries)
		r.POST("/requestvote", h.HandleVoteRequest)
		r.POST("/append", h.HandleAppendTransactionReq)
		r.POST("/transfer", h.HandleTransfer)
		r.POST("/balance", h.HandleBalance)
		r.GET("/blockchain", h.HandleGetBlockchain)

		srv := httptest.NewServer(r)
		c.servers[i] = srv
		t.Cleanup(srv.Close)
	}

	for i, node := range c.nodes {
		node.Mu.Lock()
		node.OtherNodes = make(map[int]string)
		for j, srv := range c.servers {
			if j != i {
				node.OtherNodes[j+1] = srv.URL
			}
		}
		node.Mu.Unlock()
	}

	return c
}

func (c *testCluster) forceLeader(idx int) {
	for i, node := range c.nodes {
		node.Mu.Lock()
		if i == idx {
			node.State = "Leader"
			node.CurrentTerm = 1
			node.LeaderId = node.Id
		} else {
			node.State = "Follower"
			node.CurrentTerm = 1
			node.LeaderId = c.nodes[idx].Id
		}
		node.Mu.Unlock()
	}
}

func (c *testCluster) leaderNode() *domain.Node {
	for _, n := range c.nodes {
		n.Mu.Lock()
		st := n.State
		n.Mu.Unlock()
		if st == "Leader" {
			return n
		}
	}
	return nil
}

func doPost(url string, body interface{}) (*http.Response, error) {
	data, _ := json.Marshal(body)
	return http.Post(url, "application/json", bytes.NewReader(data))
}

func TestRaft_HeartbeatFromLeader(t *testing.T) {
	c := newCluster(t)
	c.forceLeader(0)

	leader := c.nodes[0]
	follower := c.nodes[1]

	leader.Mu.Lock()
	leaderTerm := leader.CurrentTerm
	leaderID := leader.Id
	leader.Mu.Unlock()

	followerURL := c.servers[1].URL
	reqBody := structs.AppendEntriesReq{
		Term:         leaderTerm,
		LeaderID:     leaderID,
		PrevLogIndex: 0,
		PrevLogTerm:  0,
		Entries:      []structs.Transaction{},
		LeaderCommit: 0,
	}
	resp, err := doPost(followerURL+"/appendentries", reqBody)
	if err != nil {
		t.Fatalf("POST /appendentries: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var aeResp structs.AppendEntriesResp
	json.NewDecoder(resp.Body).Decode(&aeResp)
	if !aeResp.Success {
		t.Error("want success=true for valid heartbeat")
	}

	follower.Mu.Lock()
	lid := follower.LeaderId
	follower.Mu.Unlock()
	if lid != leaderID {
		t.Errorf("follower LeaderId: want %d, got %d", leaderID, lid)
	}
}

func TestRaft_LogReplication_SingleEntry(t *testing.T) {
	c := newCluster(t)
	c.forceLeader(0)

	leader := c.nodes[0]
	leader.Mu.Lock()
	leader.Log = append(leader.Log, structs.Transaction{
		ClientID: 1, Payload: "2 50", Term: 1,
	})
	leader.Mu.Unlock()

	followerURL := c.servers[1].URL
	reqBody := structs.AppendEntriesReq{
		Term:     1,
		LeaderID: leader.Id,
		Entries: []structs.Transaction{
			{ClientID: 1, Payload: "2 50", Term: 1},
		},
		LeaderCommit: 0,
	}
	resp, err := doPost(followerURL+"/appendentries", reqBody)
	if err != nil {
		t.Fatalf("POST /appendentries: %v", err)
	}
	defer resp.Body.Close()

	var aeResp structs.AppendEntriesResp
	json.NewDecoder(resp.Body).Decode(&aeResp)
	if !aeResp.Success {
		t.Error("want success=true after entry replication")
	}

	c.nodes[1].Mu.Lock()
	logLen := len(c.nodes[1].Log)
	c.nodes[1].Mu.Unlock()

	if logLen != 1 {
		t.Errorf("follower log length: want 1, got %d", logLen)
	}
}

func TestRaft_VoteRequest_CausesStepDown(t *testing.T) {
	c := newCluster(t)
	c.forceLeader(0)

	leaderURL := c.servers[0].URL
	voteReq := structs.VoteReq{
		Term:         5,
		CandidateID:  2,
		LastLogIndex: 0,
		LastLogTerm:  0,
		Timestamp:    time.Now().UnixMilli(),
	}
	resp, err := doPost(leaderURL+"/requestvote", voteReq)
	if err != nil {
		t.Fatalf("POST /requestvote: %v", err)
	}
	defer resp.Body.Close()

	var vResp structs.VoteResp
	json.NewDecoder(resp.Body).Decode(&vResp)
	if !vResp.VoteGranted {
		t.Error("want VoteGranted=true when candidate term is higher")
	}

	c.nodes[0].Mu.Lock()
	state := c.nodes[0].State
	c.nodes[0].Mu.Unlock()

	if state != "Follower" {
		t.Errorf("former leader should be Follower after higher-term vote request, got %q", state)
	}
}

func TestRaft_CommitPropagation(t *testing.T) {
	c := newCluster(t)
	c.forceLeader(0)

	entry := structs.Transaction{ClientID: 1, Payload: "2 100", Term: 1}

	for i := 1; i < 3; i++ {
		reqBody := structs.AppendEntriesReq{
			Term:         1,
			LeaderID:     c.nodes[0].Id,
			Entries:      []structs.Transaction{entry},
			LeaderCommit: 0,
		}
		resp, err := doPost(c.servers[i].URL+"/appendentries", reqBody)
		if err != nil {
			t.Fatalf("replicate to node %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	for i := 1; i < 3; i++ {
		reqBody := structs.AppendEntriesReq{
			Term:         1,
			LeaderID:     c.nodes[0].Id,
			Entries:      []structs.Transaction{},
			LeaderCommit: 1,
		}
		resp, err := doPost(c.servers[i].URL+"/appendentries", reqBody)
		if err != nil {
			t.Fatalf("commit heartbeat to node %d: %v", i+1, err)
		}
		resp.Body.Close()
	}

	for i := 1; i < 3; i++ {
		c.nodes[i].Mu.Lock()
		ci := c.nodes[i].CommitIndex
		la := c.nodes[i].LastApplied
		c.nodes[i].Mu.Unlock()

		if ci != 1 {
			t.Errorf("node %d CommitIndex: want 1, got %d", i+1, ci)
		}
		if la != 1 {
			t.Errorf("node %d LastApplied: want 1, got %d", i+1, la)
		}
	}
}

func TestRaft_LogConflict_RetrySucceeds(t *testing.T) {
	c := newCluster(t)
	c.forceLeader(0)

	followerURL := c.servers[1].URL

	c.nodes[1].Mu.Lock()
	c.nodes[1].Log = append(c.nodes[1].Log, structs.Transaction{
		ClientID: 9, Payload: "stale", Term: 1,
	})
	c.nodes[1].CurrentTerm = 1
	c.nodes[1].Mu.Unlock()

	badReq := structs.AppendEntriesReq{
		Term: 2, LeaderID: c.nodes[0].Id,
		PrevLogIndex: 1, PrevLogTerm: 2,
		Entries:      []structs.Transaction{{ClientID: 1, Payload: "correct", Term: 2}},
		LeaderCommit: 0,
	}
	resp, _ := doPost(followerURL+"/appendentries", badReq)
	var r1 structs.AppendEntriesResp
	json.NewDecoder(resp.Body).Decode(&r1)
	resp.Body.Close()
	if r1.Success {
		t.Error("want success=false on PrevLogTerm mismatch")
	}

	goodReq := structs.AppendEntriesReq{
		Term: 2, LeaderID: c.nodes[0].Id,
		PrevLogIndex: 0, PrevLogTerm: 0,
		Entries:      []structs.Transaction{{ClientID: 1, Payload: "correct", Term: 2}},
		LeaderCommit: 0,
	}
	resp2, _ := doPost(followerURL+"/appendentries", goodReq)
	var r2 structs.AppendEntriesResp
	json.NewDecoder(resp2.Body).Decode(&r2)
	resp2.Body.Close()
	if !r2.Success {
		t.Error("want success=true after corrected retry")
	}

	c.nodes[1].Mu.Lock()
	logLen := len(c.nodes[1].Log)
	payload := ""
	if logLen > 0 {
		payload = c.nodes[1].Log[0].Payload
	}
	c.nodes[1].Mu.Unlock()

	if logLen != 1 || payload != "correct" {
		t.Errorf("follower log after retry: want 1 entry 'correct', got len=%d payload=%q", logLen, payload)
	}
}

func TestRaft_Election_OneLeaderElected(t *testing.T) {
	c := newCluster(t)

	c.nodes[0].Mu.Lock()
	c.nodes[0].CurrentTerm = 0
	c.nodes[0].Mu.Unlock()

	done := make(chan struct{})
	go func() {
		c.nodes[0].StartElection()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("election did not complete within 2 s")
	}

	leaders := 0
	for _, n := range c.nodes {
		n.Mu.Lock()
		if n.State == "Leader" {
			leaders++
		}
		n.Mu.Unlock()
	}
	if leaders != 1 {
		t.Errorf("want exactly 1 leader after election, got %d", leaders)
	}

	terms := make([]int, 3)
	for i, n := range c.nodes {
		n.Mu.Lock()
		terms[i] = n.CurrentTerm
		n.Mu.Unlock()
	}
	for i := 1; i < 3; i++ {
		if terms[i] != terms[0] {
			t.Errorf("term mismatch: node 1 term=%d, node %d term=%d", terms[0], i+1, terms[i])
		}
	}
	fmt.Printf("[TC_INT_006] elected leader in term %d\n", terms[0])
}
