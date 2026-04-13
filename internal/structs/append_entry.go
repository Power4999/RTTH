package structs

type AppendEntriesReq struct {
	Term         int           `json:"term"`
	LeaderID     int           `json:"leaderid"`
	PrevLogIndex int           `json:"prevlogindex"`
	PrevLogTerm  int           `json:"prevlogterm"`
	Entries      []Transaction `json:"entries"`
	LeaderCommit int           `json:"leadercommit"`
}

type AppendEntriesResp struct {
	Term    int  `json:"term"`
	Success bool `json:"success"`
}
