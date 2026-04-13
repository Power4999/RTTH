package structs

type VoteReq struct {
	Term         int   `json:"term"`
	CandidateID  int   `json:"candidateid"`
	LastLogIndex int   `json:"lastlogindex"`
	LastLogTerm  int   `json:"lastlogterm"`
	Timestamp    int64 `json:"timestamp"`
}

type VoteResp struct {
	Term        int  `json:"term"`
	VoteGranted bool `json:"votegranted"`
}
