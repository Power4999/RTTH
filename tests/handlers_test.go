package test

import (
	"RTTH/internal/handlers"
	"RTTH/internal/store"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// setupRouter is a helper function to initialize Gin for testing
func setupRouter(h *handlers.Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.Default()
	r.POST("/append", h.HandleAppendTransactionReq)
	r.POST("/heartbeat", h.HandleHeartBeat)
	r.POST("/requestvote", h.HandleVoteRequest)
	return r
}

func TestHandleAppendTransactionReq_Leader(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	node.State = "Leader" // Force node to act as Leader

	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	payload := `{"clientid": 1, "payload": "A->B 10", "timestamp": 12345}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 OK, got %d", w.Code)
	}

	// Verify it was actually saved in the store
	if len(s.GetAll()) != 1 {
		t.Errorf("expected 1 item in store, got %d", len(s.GetAll()))
	}
}

func TestHandleAppendTransactionReq_FollowerRedirect(t *testing.T) {
	node := newIsolatedNode(t, 2, 150)
	node.State = "Follower" // Force node to act as Follower
	node.LeaderId = 1
	node.OtherNodes[1] = "http://localhost:8080"

	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	payload := `{"clientid": 1, "payload": "A->B 10", "timestamp": 12345}`
	req, _ := http.NewRequest("POST", "/append", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// A follower should not process the request, it should redirect to the leader
	if w.Code != http.StatusTemporaryRedirect {
		t.Errorf("expected HTTP 307 Temporary Redirect, got %d", w.Code)
	}
}

func TestHandleHeartBeat(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	// ASCII 64 is '@', which matches the Heartbeat rune in your domain logic
	payload := `{"leaderid": 2, "term": 1, "heartbeat": 64, "timestamp": 123456789}`
	req, _ := http.NewRequest("POST", "/heartbeat", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 OK, got %d", w.Code)
	}

	var hbResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &hbResp); err != nil {
		t.Fatalf("failed to parse heartbeat response: %v", err)
	}
	if success, ok := hbResp["success"].(bool); !ok || !success {
		t.Fatalf("expected heartbeat success=true, got %+v", hbResp)
	}
}

func TestHandleVoteRequest_OneVotePerTerm(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	firstVote := `{"term": 3, "candidateid": 2, "timestamp": 12345}`
	req1, _ := http.NewRequest("POST", "/requestvote", bytes.NewBufferString(firstVote))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for first vote, got %d", w1.Code)
	}
	var resp1 map[string]any
	if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
		t.Fatalf("failed to parse first vote response: %v", err)
	}
	if granted, ok := resp1["votegranted"].(bool); !ok || !granted {
		t.Fatalf("expected first vote to be granted, got %+v", resp1)
	}

	secondVote := `{"term": 3, "candidateid": 3, "timestamp": 12345}`
	req2, _ := http.NewRequest("POST", "/requestvote", bytes.NewBufferString(secondVote))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 OK for second vote, got %d", w2.Code)
	}
	var resp2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("failed to parse second vote response: %v", err)
	}
	if granted, ok := resp2["votegranted"].(bool); !ok || granted {
		t.Fatalf("expected second vote to be rejected, got %+v", resp2)
	}
}

func TestHandleHeartBeat_StaleTerm(t *testing.T) {
	node := newIsolatedNode(t, 1, 150)
	node.CurrentTerm = 5
	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	payload := `{"leaderid": 2, "term": 4, "heartbeat": 64, "timestamp": 123456789}`
	req, _ := http.NewRequest("POST", "/heartbeat", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for stale heartbeat response, got %d", w.Code)
	}

	var hbResp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &hbResp); err != nil {
		t.Fatalf("failed to parse stale heartbeat response: %v", err)
	}
	if success, ok := hbResp["success"].(bool); !ok || success {
		t.Fatalf("expected heartbeat success=false for stale term, got %+v", hbResp)
	}
}
