package blockchain_test

import (
	"RTTH/internal/blockchain"
	"RTTH/internal/structs"
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestBuildBlock_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "produces valid hash ending",
			run: func(t *testing.T) {
				txns := []structs.Transaction{
					{ID: 1, ClientID: 1, Payload: "2 100", Term: 1},
					{ID: 2, ClientID: 2, Payload: "3 50", Term: 1},
					{ID: 3, ClientID: 3, Payload: "1 10", Term: 1},
				}
				block := blockchain.BuildBlock(nil, txns, 1)
				if block.Hash == "" {
					t.Fatalf("BuildBlock returned empty hash")
				}
				last := block.Hash[len(block.Hash)-1]
				if last != '0' && last != '1' && last != '2' {
					t.Fatalf("PoW invariant violated: hash=%s", block.Hash)
				}
			},
		},
		{
			name: "uses zero prev hash for genesis",
			run: func(t *testing.T) {
				block := blockchain.BuildBlock(nil, []structs.Transaction{{ID: 1, ClientID: 1, Payload: "2 10", Term: 0}}, 0)
				if block.PrevHash != "0" {
					t.Fatalf("genesis PrevHash: want '0', got %q", block.PrevHash)
				}
			},
		},
		{
			name: "links next block to previous hash",
			run: func(t *testing.T) {
				txns1 := []structs.Transaction{
					{ID: 1, ClientID: 1, Payload: "2 10", Term: 1},
					{ID: 2, ClientID: 2, Payload: "3 20", Term: 1},
					{ID: 3, ClientID: 3, Payload: "1 5", Term: 1},
				}
				txns2 := []structs.Transaction{
					{ID: 4, ClientID: 1, Payload: "3 15", Term: 2},
					{ID: 5, ClientID: 2, Payload: "1 25", Term: 2},
					{ID: 6, ClientID: 3, Payload: "2 35", Term: 2},
				}
				var chain []structs.Block
				b1 := blockchain.BuildBlock(chain, txns1, 1)
				chain = append(chain, b1)
				b2 := blockchain.BuildBlock(chain, txns2, 2)
				if b2.PrevHash != b1.Hash {
					t.Fatalf("b2.PrevHash=%q, want b1.Hash=%q", b2.PrevHash, b1.Hash)
				}
			},
		},
		{
			name: "stores copy of input transactions",
			run: func(t *testing.T) {
				txns := []structs.Transaction{
					{ID: 1, ClientID: 1, Payload: "original", Term: 1},
					{ID: 2, ClientID: 2, Payload: "original", Term: 1},
					{ID: 3, ClientID: 3, Payload: "original", Term: 1},
				}
				block := blockchain.BuildBlock(nil, txns, 1)
				txns[0].Payload = "mutated"
				if block.Txns[0].Payload != "original" {
					t.Fatalf("BuildBlock must store a copy")
				}
			},
		},
		{
			name: "hash is reproducible from block data",
			run: func(t *testing.T) {
				txns := []structs.Transaction{
					{ID: 1, ClientID: 1, Payload: "2 10", Term: 1},
					{ID: 2, ClientID: 2, Payload: "3 20", Term: 1},
					{ID: 3, ClientID: 3, Payload: "1 5", Term: 1},
				}
				block := blockchain.BuildBlock(nil, txns, 1)
				var sb strings.Builder
				sb.WriteString(block.PrevHash)
				sb.WriteString(block.Nonce)
				for _, tx := range block.Txns {
					fmt.Fprintf(&sb, "%d%d%s%d%d", tx.ID, tx.ClientID, tx.Payload, tx.Timestamp, tx.Term)
				}
				want := fmt.Sprintf("%x", sha256.Sum256([]byte(sb.String())))
				if block.Hash != want {
					t.Fatalf("hash mismatch: stored=%q recomputed=%q", block.Hash, want)
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

func TestBalances_TableDriven(t *testing.T) {
	seed := []structs.Transaction{
		{ClientID: 0, Payload: "1 500", Term: 0},
		{ClientID: 0, Payload: "2 500", Term: 0},
		{ClientID: 0, Payload: "3 500", Term: 0},
	}
	seedBlock := blockchain.BuildBlock(nil, seed, 0)
	chain := []structs.Block{seedBlock}

	tests := []struct {
		name          string
		clientID      int
		uncommitted   []structs.Transaction
		wantCommitted int
		wantPending   int
	}{
		{
			name:          "empty chain committed balance",
			clientID:      1,
			uncommitted:   nil,
			wantCommitted: blockchain.GetCommittedBalance(nil, 1),
			wantPending:   blockchain.GetPendingBalance(nil, nil, 1),
		},
		{
			name:          "seeded committed balance",
			clientID:      1,
			uncommitted:   nil,
			wantCommitted: 500,
			wantPending:   500,
		},
		{
			name:     "pending includes uncommitted transfer",
			clientID: 1,
			uncommitted: []structs.Transaction{
				{ClientID: 1, Payload: "2 100", Term: 1},
			},
			wantCommitted: 500,
			wantPending:   400,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			localChain := chain
			if strings.Contains(tt.name, "empty chain") {
				localChain = nil
			}
			gotCommitted := blockchain.GetCommittedBalance(localChain, tt.clientID)
			gotPending := blockchain.GetPendingBalance(localChain, tt.uncommitted, tt.clientID)
			if gotCommitted != tt.wantCommitted {
				t.Fatalf("committed: want %d got %d", tt.wantCommitted, gotCommitted)
			}
			if gotPending != tt.wantPending {
				t.Fatalf("pending: want %d got %d", tt.wantPending, gotPending)
			}
		})
	}
}

func TestLoadFirstBlockchain_TableDriven(t *testing.T) {
	makeSeedFile := func(t *testing.T, contents string) string {
		t.Helper()
		path := t.TempDir() + "/seed.txt"
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatalf("write seed file: %v", err)
		}
		return path
	}

	tests := []struct {
		name       string
		path       string
		wantErr    bool
		wantBlocks int
	}{
		{
			name:       "missing file returns error",
			path:       "/nonexistent/path.txt",
			wantErr:    true,
			wantBlocks: 0,
		},
		{
			name:       "parses seed file",
			path:       makeSeedFile(t, "0 1 1000\n0 2 1000\n0 3 1000\n"),
			wantErr:    false,
			wantBlocks: 1,
		},
		{
			name:       "ignores comments in seed file",
			path:       makeSeedFile(t, "# seed file\n\n0 1 1000\n# ignore\n0 2 1000\n0 3 1000\n"),
			wantErr:    false,
			wantBlocks: 1,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			chain, err := blockchain.LoadFirstBlockchain(tt.path)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadFirstBlockchain: %v", err)
			}
			if len(chain) < tt.wantBlocks {
				t.Fatalf("expected at least %d blocks, got %d", tt.wantBlocks, len(chain))
			}
			for i, b := range chain {
				last := b.Hash[len(b.Hash)-1]
				if last != '0' && last != '1' && last != '2' {
					t.Fatalf("block %d: invalid PoW hash %s", i, b.Hash)
				}
			}
		})
	}
}

func TestPrintChain_TableDriven(t *testing.T) {
	tests := []struct {
		name  string
		chain []structs.Block
		check func(t *testing.T, out string)
	}{
		{
			name:  "empty chain",
			chain: nil,
			check: func(t *testing.T, out string) {
				if out != "[]" {
					t.Fatalf("want [] got %q", out)
				}
			},
		},
		{
			name: "non empty chain",
			chain: func() []structs.Block {
				txns := []structs.Transaction{
					{ClientID: 1, Payload: "2 10", Term: 3},
					{ClientID: 2, Payload: "3 20", Term: 3},
					{ClientID: 3, Payload: "1 5", Term: 3},
				}
				return []structs.Block{blockchain.BuildBlock(nil, txns, 3)}
			}(),
			check: func(t *testing.T, out string) {
				if out == "[]" {
					t.Fatalf("expected non-empty output")
				}
				if !strings.Contains(out, "t=3") {
					t.Fatalf("expected term info in output, got %q", out)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out := blockchain.PrintChain(tt.chain)
			tt.check(t, out)
		})
	}
}
