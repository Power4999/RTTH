package structs

type Transaction struct {
	ID        int    `json:"id"`
	ClientID  int    `json:"clientid"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
	Term      int    `json:"term"`
}
