package handlers_test

import (
	"RTTH/internal/domain"
	"RTTH/internal/handlers"
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupRouter(h *handlers.Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/append", h.HandleAppendTransactionReq)
	r.POST("/appendentries", h.HandleAppendEntries)
	r.POST("/requestvote", h.HandleVoteRequest)
	r.POST("/getuserdetails", h.GetUserDetails)
	r.GET("/getalluserdetails", h.GetAllUserDetails)
	r.POST("/transfer", h.HandleTransfer)
	r.POST("/balance", h.HandleBalance)
	r.GET("/blockchain", h.HandleGetBlockchain)
	return r
}

func newTestNode(t *testing.T, id int) *domain.Node {
	t.Helper()
	node, err := domain.NewNode(id, 150, t.TempDir())
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	return node
}

func doPost(router *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doGet(router *gin.Engine, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestHandleAppendTransactionReq_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*domain.Node)
		body         string
		wantCode     int
		wantLocation string
	}{
		{
			name: "leader accepts valid append",
			setup: func(n *domain.Node) {
				n.State = "Leader"
			},
			body:     `{"clientid":1,"payload":"A->B 10","timestamp":12345}`,
			wantCode: http.StatusOK,
		},
		{
			name: "follower redirects to leader",
			setup: func(n *domain.Node) {
				n.State = "Follower"
				n.LeaderId = 1
				n.OtherNodes[1] = "http://localhost:8081"
			},
			body:         `{"clientid":1,"payload":"A->B 10","timestamp":12345}`,
			wantCode:     http.StatusTemporaryRedirect,
			wantLocation: "http://localhost:8081/append",
		},
		{
			name: "candidate returns unavailable",
			setup: func(n *domain.Node) {
				n.State = "Candidate"
			},
			body:     `{"clientid":1,"payload":"A->B 10","timestamp":12345}`,
			wantCode: http.StatusServiceUnavailable,
		},
		{
			name: "leader rejects malformed json",
			setup: func(n *domain.Node) {
				n.State = "Leader"
			},
			body:     `not json`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			node := newTestNode(t, 1)
			node.Mu.Lock()
			tt.setup(node)
			node.Mu.Unlock()
			s := store.NewMemoryStore()
			router := setupRouter(handlers.NewHandler(s, node))
			w := doPost(router, "/append", tt.body)
			if w.Code != tt.wantCode {
				t.Fatalf("want %d got %d body=%s", tt.wantCode, w.Code, w.Body.String())
			}
			if tt.wantLocation != "" && w.Header().Get("Location") != tt.wantLocation {
				t.Fatalf("want Location %q got %q", tt.wantLocation, w.Header().Get("Location"))
			}
		})
	}
}

func TestRaftRPCHandlers_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		body     string
		setup    func(*domain.Node)
		wantCode int
		check    func(t *testing.T, node *domain.Node, body []byte)
	}{
		{
			name:     "append entries heartbeat succeeds",
			path:     "/appendentries",
			body:     `{"term":1,"leaderid":2,"prevlogindex":0,"prevlogterm":0,"entries":[],"leadercommit":0}`,
			setup:    func(n *domain.Node) {},
			wantCode: http.StatusOK,
			check:    func(t *testing.T, node *domain.Node, body []byte) {},
		},
		{
			name:     "vote request stale term rejected",
			path:     "/requestvote",
			body:     `{"term":3,"candidateid":2,"lastlogindex":0,"lastlogterm":0,"timestamp":0}`,
			setup:    func(n *domain.Node) { n.CurrentTerm = 5 },
			wantCode: http.StatusOK,
			check: func(t *testing.T, node *domain.Node, body []byte) {
				var resp map[string]interface{}
				_ = json.Unmarshal(body, &resp)
				if granted, _ := resp["votegranted"].(bool); granted {
					t.Fatalf("expected votegranted=false")
				}
			},
		},
		{
			name:     "vote request malformed json",
			path:     "/requestvote",
			body:     `bad`,
			setup:    func(n *domain.Node) {},
			wantCode: http.StatusBadRequest,
			check:    func(t *testing.T, node *domain.Node, body []byte) {},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			node := newTestNode(t, 1)
			node.Mu.Lock()
			tt.setup(node)
			node.Mu.Unlock()
			router := setupRouter(handlers.NewHandler(store.NewMemoryStore(), node))
			w := doPost(router, tt.path, tt.body)
			if w.Code != tt.wantCode {
				t.Fatalf("want %d got %d body=%s", tt.wantCode, w.Code, w.Body.String())
			}
			tt.check(t, node, w.Body.Bytes())
		})
	}
}

func TestReadHandlers_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		setup    func(*store.MemoryStore, *domain.Node)
		wantCode int
	}{
		{
			name:   "get user details found",
			method: http.MethodPost,
			path:   "/getuserdetails",
			body:   `{"clientid":1}`,
			setup: func(s *store.MemoryStore, n *domain.Node) {
				_ = s.Append(structs.Transaction{ClientID: 7, Payload: "A->B 100"})
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "get user details missing",
			method:   http.MethodPost,
			path:     "/getuserdetails",
			body:     `{"clientid":999}`,
			setup:    func(s *store.MemoryStore, n *domain.Node) {},
			wantCode: http.StatusNotFound,
		},
		{
			name:   "get all user details",
			method: http.MethodGet,
			path:   "/getalluserdetails",
			setup: func(s *store.MemoryStore, n *domain.Node) {
				_ = s.Append(structs.Transaction{ClientID: 1, Payload: "A->B 10"})
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "balance missing client id",
			method:   http.MethodPost,
			path:     "/balance",
			body:     `{"clientid":0}`,
			setup:    func(s *store.MemoryStore, n *domain.Node) {},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "blockchain endpoint returns array",
			method:   http.MethodGet,
			path:     "/blockchain",
			setup:    func(s *store.MemoryStore, n *domain.Node) {},
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			s := store.NewMemoryStore()
			n := newTestNode(t, 1)
			tt.setup(s, n)
			router := setupRouter(handlers.NewHandler(s, n))
			var w *httptest.ResponseRecorder
			if tt.method == http.MethodGet {
				w = doGet(router, tt.path)
			} else {
				w = doPost(router, tt.path, tt.body)
			}
			if w.Code != tt.wantCode {
				t.Fatalf("want %d got %d body=%s", tt.wantCode, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleTransfer_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(*domain.Node)
		body         string
		wantCode     int
		wantLocation string
	}{
		{
			name: "leader accepts valid transfer",
			setup: func(n *domain.Node) {
				n.Mu.Lock()
				n.State = "Leader"
				n.CurrentTerm = 1
				n.Mu.Unlock()
				go func() {
					for {
						n.Mu.Lock()
						if len(n.Log) > 0 && n.CommitIndex == 0 {
							n.CommitIndex = 1
							n.LastApplied = 0
							n.Mu.Unlock()
							return
						}
						n.Mu.Unlock()
					}
				}()
			},
			body:     `{"clientid":1,"payload":"2 50","timestamp":99999}`,
			wantCode: http.StatusOK,
		},
		{
			name: "follower redirects transfer",
			setup: func(n *domain.Node) {
				n.Mu.Lock()
				n.State = "Follower"
				n.LeaderId = 1
				n.OtherNodes[1] = "http://localhost:8081"
				n.Mu.Unlock()
			},
			body:         `{"clientid":1,"payload":"2 50","timestamp":12345}`,
			wantCode:     http.StatusTemporaryRedirect,
			wantLocation: "http://localhost:8081/transfer",
		},
		{
			name: "candidate returns unavailable",
			setup: func(n *domain.Node) {
				n.Mu.Lock()
				n.State = "Candidate"
				n.Mu.Unlock()
			},
			body:     `{"clientid":1,"payload":"2 50","timestamp":12345}`,
			wantCode: http.StatusServiceUnavailable,
		},
		{
			name: "invalid transfer payload",
			setup: func(n *domain.Node) {
				n.Mu.Lock()
				n.State = "Leader"
				n.Mu.Unlock()
			},
			body:     `{"clientid":1,"payload":"","timestamp":12345}`,
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			node := newTestNode(t, 1)
			tt.setup(node)
			router := setupRouter(handlers.NewHandler(store.NewMemoryStore(), node))
			w := doPost(router, "/transfer", tt.body)
			if w.Code != tt.wantCode {
				t.Fatalf("want %d got %d body=%s", tt.wantCode, w.Code, w.Body.String())
			}
			if tt.wantLocation != "" && w.Header().Get("Location") != tt.wantLocation {
				t.Fatalf("want Location %q got %q", tt.wantLocation, w.Header().Get("Location"))
			}
		})
	}
}
