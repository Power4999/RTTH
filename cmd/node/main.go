package main

import (
	"RTTH/internal/handlers"
	"fmt"
	"log"
	"os"
	"strconv"

	//"RTTH/internal/store"
	"RTTH/internal/domain"

	"github.com/gin-gonic/gin"
)

// go run main.go nodeId portNo timeout
func main() {
	if len(os.Args) < 4 {
		log.Fatalf("usage: go run main.go <nodeId> <portNo> <timeoutMs>")
	}

	nodeId, err := strconv.Atoi(os.Args[1])
	if err != nil || nodeId <= 0 {
		log.Fatalf("invalid nodeId: %q", os.Args[1])
	}

	nodePort, err := strconv.Atoi(os.Args[2])
	if err != nil || nodePort <= 0 {
		log.Fatalf("invalid portNo: %q", os.Args[2])
	}

	nodeTimeout, err := strconv.Atoi(os.Args[3])
	if err != nil || nodeTimeout <= 0 {
		log.Fatalf("invalid timeoutMs: %q", os.Args[3])
	}

	RaftNode := domain.NewNode(nodeId, nodeTimeout)

	handler := handlers.NewHandler(RaftNode.Store, RaftNode)
	router := gin.Default()
	go RaftNode.Run()
	router.POST("/append", handler.HandleAppendTransactionReq)
	router.POST("/heartbeat", handler.HandleHeartBeat)
	router.POST("/requestvote", handler.HandleVoteRequest)
	router.POST("/getuserdetails", handler.GetUserDetails)
	router.POST("/getalluserdetails", handler.GetAllUserDetails)
	if err := router.Run(fmt.Sprintf(":%d", nodePort)); err != nil {
		log.Fatal(err)
	}
}
