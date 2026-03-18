package structs

type HeartBeat struct {
	LeaderID  int   `json:"leaderid"`
	Term      int   `json:"term"`
	Heartbeat rune  `json:"heartbeat"`
	Timestamp int64 `json:"timestamp"`
}

type HeartBeatResp struct {
	Term    int  `json:"term"`
	Success bool `json:"success"`
}
