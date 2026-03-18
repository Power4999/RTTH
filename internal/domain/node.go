package domain

import (
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"bytes"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	defaultElectionTimeoutMs = 150
	defaultDataDir           = ".raft"
)

type persistedState struct {
	CurrentTerm int `json:"current_term"`
	VotedFor    int `json:"voted_for"`
}

type Node struct {
	Mu sync.Mutex

	Id                  int
	State               string
	Store               *store.MemoryStore
	OtherNodes          map[int]string ////map[id]url
	LeaderId            int
	LastLeaderTimeStamp int64
	Timeout             int
	CurrentTerm         int
	VotedFor            int
	rng                 *rand.Rand
	DataDir             string
}

func NewNode(id int, timeout int) *Node {
	node := &Node{
		Id:    id,
		State: "Follower",
		Store: store.NewMemoryStore(),
		OtherNodes: map[int]string{
			1: "http://localhost:8080",
			2: "http://localhost:8081",
			3: "http://localhost:8082",
		},
		LeaderId:            0,
		LastLeaderTimeStamp: 0,
		Timeout:             timeout,
		CurrentTerm:         0,
		VotedFor:            0,
		rng:                 rand.New(rand.NewSource(time.Now().UnixNano() + int64(id))),
		DataDir:             defaultDataDir,
	}

	if customDir := os.Getenv("RAFT_DATA_DIR"); customDir != "" {
		node.DataDir = customDir
	}

	node.loadState()
	return node
}

func (n *Node) stateFilePath() string {
	return filepath.Join(n.DataDir, "node_"+strconv.Itoa(n.Id)+".json")
}

func (n *Node) loadState() {
	if err := os.MkdirAll(n.DataDir, 0o755); err != nil {
		log.Printf("failed to create raft data directory %s: %v", n.DataDir, err)
		return
	}

	statePath := n.stateFilePath()
	raw, err := os.ReadFile(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to read raft state file %s: %v", statePath, err)
		}
		return
	}

	var st persistedState
	if err := json.Unmarshal(raw, &st); err != nil {
		log.Printf("failed to unmarshal raft state for node %d: %v", n.Id, err)
		return
	}

	n.CurrentTerm = st.CurrentTerm
	n.VotedFor = st.VotedFor
}

func (n *Node) persistStateLocked() {
	if err := os.MkdirAll(n.DataDir, 0o755); err != nil {
		log.Printf("failed to create raft data directory %s: %v", n.DataDir, err)
		return
	}

	st := persistedState{CurrentTerm: n.CurrentTerm, VotedFor: n.VotedFor}
	raw, err := json.Marshal(st)
	if err != nil {
		log.Printf("failed to marshal raft state for node %d: %v", n.Id, err)
		return
	}

	statePath := n.stateFilePath()
	if err := os.WriteFile(statePath, raw, 0o644); err != nil {
		log.Printf("failed to persist raft state for node %d: %v", n.Id, err)
	}
}

func (n *Node) electionTimeoutMsLocked() int64 {
	timeout := n.Timeout
	if timeout <= 0 {
		timeout = defaultElectionTimeoutMs
	}
	// Randomized timeout in [timeout, 2*timeout] reduces split votes.
	return int64(timeout + n.rng.Intn(timeout+1))
}

func (n *Node) resetElectionTimerLocked() {
	n.LastLeaderTimeStamp = time.Now().UnixMilli() + n.electionTimeoutMsLocked()
}

func (n *Node) Run() {

	n.Mu.Lock()
	delete(n.OtherNodes, n.Id)
	n.State = "Follower"
	n.LeaderId = 0
	n.resetElectionTimerLocked()
	n.Mu.Unlock()

	for {
		n.Mu.Lock()
		state := n.State
		electionDeadline := n.LastLeaderTimeStamp
		n.Mu.Unlock()

		switch state {
		case "Leader":
			time.Sleep(100 * time.Millisecond)
			n.SendHeartBeat()
		case "Follower", "Candidate":
			time.Sleep(25 * time.Millisecond)
			if time.Now().UnixMilli() >= electionDeadline {
				n.StartElection()
			}
		}
	}
}

// major change
func (nodes *Node) SendHeartBeat() {

	nodes.Mu.Lock()
	if nodes.State != "Leader" {
		nodes.Mu.Unlock()
		return
	}
	id := nodes.Id
	term := nodes.CurrentTerm
	var peerURLs []string
	for _, url := range nodes.OtherNodes {
		peerURLs = append(peerURLs, url)
	}

	nodes.Mu.Unlock()

	heartbeatMsg := structs.HeartBeat{
		LeaderID:  id,
		Term:      term,
		Heartbeat: '@',
		Timestamp: time.Now().UnixMilli(),
	}
	jsonData, err := json.Marshal(heartbeatMsg)
	if err != nil {
		log.Println("failed to marshal heartbeat:", err)
		return
	}
	// timeout
	client := &http.Client{Timeout: 150 * time.Millisecond}

	for _, targetURL := range peerURLs {
		go func(url string) {
			resp, err := client.Post(url+"/heartbeat", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var hbResp structs.HeartBeatResp
			if err := json.NewDecoder(resp.Body).Decode(&hbResp); err != nil {
				return
			}

			nodes.Mu.Lock()
			defer nodes.Mu.Unlock()
			if hbResp.Term > nodes.CurrentTerm {
				nodes.CurrentTerm = hbResp.Term
				nodes.State = "Follower"
				nodes.VotedFor = 0
				nodes.LeaderId = 0
				nodes.resetElectionTimerLocked()
				nodes.persistStateLocked()
			}
		}(targetURL)
	}
}

func (node *Node) StartElection() {
	node.Mu.Lock()
	if node.State == "Leader" {
		node.Mu.Unlock()
		return
	}

	node.State = "Candidate"
	node.CurrentTerm++
	node.resetElectionTimerLocked()
	node.VotedFor = node.Id
	node.LeaderId = 0
	node.persistStateLocked()

	term := node.CurrentTerm
	id := node.Id
	var peerURLs []string
	for _, url := range node.OtherNodes {
		peerURLs = append(peerURLs, url)
	}
	requiredVotes := (len(node.OtherNodes)+1)/2 + 1

	node.Mu.Unlock()
	votesReceived := 1

	voteReq := structs.VoteReq{
		Term:        term,
		CandidateID: id,
		Timestamp:   time.Now().UnixMilli(),
	}
	jsonData, err := json.Marshal(voteReq)
	if err != nil {
		log.Println("failed to marshal vote request:", err)
		return
	}

	voteCh := make(chan structs.VoteResp, len(peerURLs))

	client := &http.Client{Timeout: 200 * time.Millisecond}

	for _, peer := range peerURLs {
		go func(url string) {
			resp, err := client.Post(url+"/requestvote", "application/json", bytes.NewBuffer(jsonData))
			if err != nil {
				voteCh <- structs.VoteResp{Term: term, VoteGranted: false}
				return
			}
			defer resp.Body.Close()

			var voteResp structs.VoteResp
			if err := json.NewDecoder(resp.Body).Decode(&voteResp); err != nil {
				voteCh <- structs.VoteResp{Term: term, VoteGranted: false}
				return
			}
			voteCh <- voteResp
		}(peer)
	}

	becameLeader := false

	for i := 0; i < len(peerURLs); i++ {
		voteResp := <-voteCh

		node.Mu.Lock()
		if voteResp.Term > node.CurrentTerm {
			node.CurrentTerm = voteResp.Term
			node.State = "Follower"
			node.VotedFor = 0
			node.LeaderId = 0
			node.resetElectionTimerLocked()
			node.persistStateLocked()
			node.Mu.Unlock()
			return
		}
		if node.State != "Candidate" || node.CurrentTerm != term {
			node.Mu.Unlock()
			return
		}

		if voteResp.VoteGranted {
			votesReceived++
		}

		if votesReceived >= requiredVotes && !becameLeader {
			node.State = "Leader"
			node.LeaderId = node.Id
			becameLeader = true
		}
		node.Mu.Unlock()
	}

	if becameLeader {
		node.SendHeartBeat()
		return
	}

	node.Mu.Lock()
	if node.State == "Candidate" && node.CurrentTerm == term {
		node.State = "Follower"
		node.LeaderId = 0
		node.resetElectionTimerLocked()
	}
	node.Mu.Unlock()
}

func (n *Node) ProcessVoteRequest(voteReq structs.VoteReq) structs.VoteResp {
	voteGranted := false

	n.Mu.Lock()
	defer n.Mu.Unlock()

	if voteReq.Term < n.CurrentTerm {
		return structs.VoteResp{Term: n.CurrentTerm, VoteGranted: false}
	}

	if voteReq.Term > n.CurrentTerm {
		n.CurrentTerm = voteReq.Term
		n.State = "Follower"
		n.VotedFor = 0
		n.LeaderId = 0
		n.persistStateLocked()
	}

	votedFor := n.VotedFor
	if votedFor == 0 || votedFor == voteReq.CandidateID {
		voteGranted = true
		n.VotedFor = voteReq.CandidateID
		n.resetElectionTimerLocked()
		n.persistStateLocked()
	}

	return structs.VoteResp{Term: n.CurrentTerm, VoteGranted: voteGranted}
}

func (n *Node) ProcessHeartbeat(heartbeat structs.HeartBeat) bool {
	n.Mu.Lock()
	defer n.Mu.Unlock()

	if heartbeat.Term < n.CurrentTerm {
		return false
	}

	if heartbeat.Term > n.CurrentTerm {
		n.CurrentTerm = heartbeat.Term
		n.VotedFor = 0
		n.persistStateLocked()
	}

	n.State = "Follower"
	n.LeaderId = heartbeat.LeaderID
	n.resetElectionTimerLocked()
	return true
}
