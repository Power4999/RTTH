package handlers

import (
	"RTTH/internal/domain"
	"RTTH/internal/store"
	"RTTH/internal/structs"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	Store    store.LogStore
	RaftNode *domain.Node
}

// NewHandler performs handler wiring and returns a handler bound to store and RAFT node.
func NewHandler(s store.LogStore, n *domain.Node) *Handler {
	return &Handler{Store: s, RaftNode: n}
}

// HandleAppendTransactionReq performs legacy append request handling and returns an HTTP response.
func (h *Handler) HandleAppendTransactionReq(c *gin.Context) {
	node := h.RaftNode
	var txn structs.ClientTransaction
	if err := c.ShouldBindJSON(&txn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node.Mu.Lock()
	state := node.State
	leaderURL := node.OtherNodes[node.LeaderId]
	node.Mu.Unlock()

	switch state {
	case "Leader":
		if err := txn.Validate(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		entry := structs.Transaction{
			ClientID:  txn.ClientID,
			Payload:   txn.Payload,
			Timestamp: txn.Timestamp,
		}
		if err := h.Store.Append(entry); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "successfully stored"})

	case "Follower":
		if leaderURL == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no known leader, election in progress"})
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, leaderURL+"/append")

	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "election in progress, no leader available"})
	}
}

// HandleVoteRequest performs RAFT vote request handling and returns an HTTP response.
func (h *Handler) HandleVoteRequest(c *gin.Context) {
	var req structs.VoteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.RaftNode.ProcessVoteRequest(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// HandleAppendEntries performs RAFT append entries handling and returns an HTTP response.
func (h *Handler) HandleAppendEntries(c *gin.Context) {
	var req structs.AppendEntriesReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := h.RaftNode.ProcessAppendEntries(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "storage error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetUserDetails performs transaction lookup by client ID and returns an HTTP response.
func (h *Handler) GetUserDetails(c *gin.Context) {
	var req structs.GetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "clientid is required"})
		return
	}
	txn, err := h.Store.GetByID(req.ClientID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, txn)
}

// GetAllUserDetails performs retrieval of all stored transactions and returns an HTTP response.
func (h *Handler) GetAllUserDetails(c *gin.Context) {
	c.JSON(http.StatusOK, h.Store.GetAll())
}

// HandleTransfer performs transfer submission through RAFT and returns commit and balance results.
func (h *Handler) HandleTransfer(c *gin.Context) {
	var txn structs.ClientTransaction
	if err := c.ShouldBindJSON(&txn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := txn.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	node := h.RaftNode
	node.Mu.Lock()
	state := node.State
	leaderURL := node.OtherNodes[node.LeaderId]
	node.Mu.Unlock()

	switch state {
	case "Leader":
		idx, err := node.AppendTransaction(txn.ClientID, txn.Payload, txn.Timestamp)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		committed := node.WaitForCommit(idx, 3*time.Second)
		committedBal, pendingBal := node.GetBalance(txn.ClientID)
		c.JSON(http.StatusOK, gin.H{
			"committed":         committed,
			"committed_balance": committedBal,
			"pending_balance":   pendingBal,
		})

	case "Follower":
		if leaderURL == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no known leader, election in progress"})
			return
		}
		c.Redirect(http.StatusTemporaryRedirect, leaderURL+"/transfer")

	default:
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "election in progress, no leader available"})
	}
}

// HandleBalance performs balance lookup for a client and returns committed and pending values.
func (h *Handler) HandleBalance(c *gin.Context) {
	var req structs.GetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.ClientID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "clientid is required"})
		return
	}
	committedBal, pendingBal := h.RaftNode.GetBalance(req.ClientID)
	c.JSON(http.StatusOK, gin.H{
		"client_id":         req.ClientID,
		"committed_balance": committedBal,
		"pending_balance":   pendingBal,
	})
}

// HandleGetBlockchain performs blockchain snapshot retrieval and returns the current chain.
func (h *Handler) HandleGetBlockchain(c *gin.Context) {
	c.JSON(http.StatusOK, h.RaftNode.GetBlockchain())
}
