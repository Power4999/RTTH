package test

import (
	"RTTH/internal/domain"
	"RTTH/internal/handlers"
	"RTTH/internal/store"
	"bytes"
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
	return r
}

func TestHandleAppendTransactionReq_Leader(t *testing.T) {
	node := domain.NewNode(1, 150)
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
	node := domain.NewNode(2, 150)
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
	node := domain.NewNode(1, 150)
	s := store.NewMemoryStore()
	h := handlers.NewHandler(s, node)
	router := setupRouter(h)

	// ASCII 64 is '@', which matches the Heartbeat rune in your domain logic
	payload := `{"leaderid": 2, "heartbeat": 64, "timestamp": 123456789}` 
	req, _ := http.NewRequest("POST", "/heartbeat", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 OK, got %d", w.Code)
	}
}