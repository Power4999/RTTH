package main

import (
	"RTTH/internal/handlers"
	"os"
	"strconv"

	//"RTTH/internal/store"
	"RTTH/internal/domain"

	"github.com/gin-gonic/gin"
)

// go run main.go nodeId portNo timeout
func main() {
	// hard coded 1 node
	nodeId, _ := strconv.Atoi(os.Args[1])
	nodeTimeout, _ := strconv.Atoi(os.Args[3])
	RaftNode := domain.NewNode(nodeId, nodeTimeout)

	handler := handlers.NewHandler(RaftNode.Store, RaftNode)
	router := gin.Default()
	go RaftNode.Run()
	router.POST("/append", handler.HandleAppendTransactionReq)
	router.POST("/heartbeat", handler.HandleHeartBeat)
	router.POST("/requestvote", handler.HandleVoteRequest)
	router.POST("/getuserdetails", handler.GetUserDetails)
	router.POST("/getalluserdetails", handler.GetAllUserDetails)
	router.Run(":" + os.Args[2])
}
