package structs

const BlockSize = 3

type Block struct {
	Term     int           `json:"term"`
	PrevHash string        `json:"prev_hash"`
	Nonce    string        `json:"nonce"`
	Txns     []Transaction `json:"txns"`
	Hash     string        `json:"hash"`
}
