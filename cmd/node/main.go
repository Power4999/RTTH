package main

import (
	"RTTH/internal/domain"
	"RTTH/internal/handlers"
	"fmt"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) < 5 {
		fmt.Fprintln(os.Stderr, "usage: main <nodeID> <client_port> <server_port> <timeoutMs> <dataDir>")
		os.Exit(1)
	}

	nodeID, err := strconv.Atoi(os.Args[1])
	if err != nil || nodeID <= 0 {
		fmt.Fprintln(os.Stderr, "nodeID must be a positive integer")
		os.Exit(1)
	}

	clientPort := os.Args[2]

	nodeTimeout, err := strconv.Atoi(os.Args[3])
	if err != nil || nodeTimeout <= 0 {
		fmt.Fprintln(os.Stderr, "timeoutMs must be a positive integer")
		os.Exit(1)
	}

	dataDir := os.Args[4]

	raftNode, err := domain.NewNode(nodeID, nodeTimeout, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialise node: %v\n", err)
		os.Exit(1)
	}

	handler := handlers.NewHandler(raftNode.Store, raftNode)
	router := gin.Default()
	go raftNode.Run()

	router.POST("/appendentries", handler.HandleAppendEntries)
	router.POST("/requestvote", handler.HandleVoteRequest)

	router.POST("/append", handler.HandleAppendTransactionReq)
	router.POST("/getuserdetails", handler.GetUserDetails)
	router.GET("/getalluserdetails", handler.GetAllUserDetails)

	router.POST("/transfer", handler.HandleTransfer)
	router.POST("/balance", handler.HandleBalance)
	router.GET("/blockchain", handler.HandleGetBlockchain)

	router.Run(":" + clientPort)
}
