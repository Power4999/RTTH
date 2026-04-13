package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const channelBase = "http://localhost:9000/forward"

var nodeURLs = map[int]string{
	1: channelBase + "/1",
	2: channelBase + "/2",
	3: channelBase + "/3",
}

type transferReq struct {
	ClientID  int    `json:"clientid"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

type balanceReq struct {
	ClientID int `json:"clientid"`
}

var httpClient = &http.Client{
	Timeout: 4 * time.Second,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

func postJSON(url string, payload interface{}) (map[string]interface{}, int, string, error) {
	data, _ := json.Marshal(payload)
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, 0, "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	loc := resp.Header.Get("Location")
	var result map[string]interface{}
	_ = json.Unmarshal(raw, &result)
	return result, resp.StatusCode, loc, nil
}

func doTransfer(clientID, receiver, amount int) {
	payload := fmt.Sprintf("%d %d", receiver, amount)
	req := transferReq{
		ClientID:  clientID,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}

	for attempt := 1; attempt <= 5; attempt++ {
		fmt.Printf("  [attempt %d] trying nodes...\n", attempt)
		for nodeID := 1; nodeID <= 3; nodeID++ {
			url := nodeURLs[nodeID] + "/transfer"
			result, code, loc, err := postJSON(url, req)
			if err != nil {
				fmt.Printf("    node %d: network error: %v\n", nodeID, err)
				continue
			}
			switch code {
			case http.StatusOK:
				fmt.Printf("  ✓ Transfer submitted (committed=%v)\n", result["committed"])
				fmt.Printf("    Committed balance: %v | Pending balance: %v\n",
					result["committed_balance"], result["pending_balance"])
				return
			case http.StatusTemporaryRedirect:

				if loc != "" {
					result2, code2, _, err2 := postJSON(loc, req)
					if err2 == nil && code2 == http.StatusOK {
						fmt.Printf("  ✓ Transfer submitted via leader redirect (committed=%v)\n", result2["committed"])
						fmt.Printf("    Committed balance: %v | Pending balance: %v\n",
							result2["committed_balance"], result2["pending_balance"])
						return
					}
				}
			case http.StatusServiceUnavailable:
				fmt.Printf("    node %d: unavailable (election in progress)\n", nodeID)
			default:
				fmt.Printf("    node %d: HTTP %d — %v\n", nodeID, code, result)
			}
		}
		time.Sleep(time.Duration(300+attempt*200) * time.Millisecond)
	}
	fmt.Println("  ✗ Transfer failed: no leader available after retries.")
}

func doBalance(clientID int) {
	req := balanceReq{ClientID: clientID}
	for nodeID := 1; nodeID <= 3; nodeID++ {
		url := nodeURLs[nodeID] + "/balance"
		result, code, _, err := postJSON(url, req)
		if err != nil {
			fmt.Printf("    node %d: network error: %v\n", nodeID, err)
			continue
		}
		if code == http.StatusOK {
			fmt.Printf("  Client %d — Committed: %v | Pending: %v\n",
				clientID, result["committed_balance"], result["pending_balance"])
			return
		}
		fmt.Printf("    node %d: HTTP %d — %v\n", nodeID, code, result)
	}
	fmt.Println("  ✗ Balance check failed.")
}

func doBlockchain() {
	for nodeID := 1; nodeID <= 3; nodeID++ {
		url := nodeURLs[nodeID] + "/blockchain"
		resp, err := httpClient.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		var chain []map[string]interface{}
		if err := json.Unmarshal(raw, &chain); err != nil {
			continue
		}
		fmt.Printf("  Blockchain from node %d (%d blocks):\n", nodeID, len(chain))
		for i, b := range chain {
			fmt.Printf("    Block %d | term=%.0f | hash=...%s\n",
				i+1, b["term"],
				shortHash(fmt.Sprintf("%v", b["hash"])))
			if txns, ok := b["txns"].([]interface{}); ok {
				for _, t := range txns {
					fmt.Printf("      %v\n", t)
				}
			}
		}
		return
	}
	fmt.Println("  ✗ Could not retrieve blockchain.")
}

func shortHash(h string) string {
	if len(h) > 8 {
		return h[len(h)-8:]
	}
	return h
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)

	clientID := 0
	if len(os.Args) > 1 {
		clientID, _ = strconv.Atoi(os.Args[1])
	}
	for clientID < 1 || clientID > 3 {
		fmt.Print("Enter your client ID (1–3): ")
		scanner.Scan()
		clientID, _ = strconv.Atoi(strings.TrimSpace(scanner.Text()))
	}
	fmt.Printf("Logged in as client %d\n", clientID)

	for {
		fmt.Printf("\n── Banking Menu (client %d) ──────────────────\n", clientID)
		fmt.Println("  1) Transfer funds")
		fmt.Println("  2) Check my balance")
		fmt.Println("  3) View blockchain")
		fmt.Println("  4) Exit")
		fmt.Print("Choice: ")

		scanner.Scan()
		switch strings.TrimSpace(scanner.Text()) {
		case "1":
			fmt.Print("  Receiver ID (1–3): ")
			scanner.Scan()
			receiver, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if err != nil || receiver < 1 || receiver > 3 {
				fmt.Println("  Invalid receiver ID.")
				continue
			}
			if receiver == clientID {
				fmt.Println("  Cannot transfer to yourself.")
				continue
			}
			fmt.Print("  Amount: ")
			scanner.Scan()
			amount, err := strconv.Atoi(strings.TrimSpace(scanner.Text()))
			if err != nil || amount <= 0 {
				fmt.Println("  Invalid amount.")
				continue
			}
			doTransfer(clientID, receiver, amount)

		case "2":
			doBalance(clientID)

		case "3":
			doBlockchain()

		case "4":
			fmt.Println("Goodbye!")
			return

		default:
			fmt.Println("  Unknown option.")
		}
	}
}
