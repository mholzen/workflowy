package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type RequestPayload struct {
	ItemID string `json:"item_id"`
}

func fatalIfError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}

func getAPIKey() (string, error) {
	apiKeyBytes, err := os.ReadFile(".api-key")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(apiKeyBytes)), nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <item_id>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	itemID := os.Args[1]
	request := RequestPayload{
		ItemID: itemID,
	}

	// send request
	jsonPayload, err := json.Marshal(request)
	fatalIfError(err, "Error marshaling JSON")

	req, err := http.NewRequest("POST", "https://beta.workflowy.com/api/beta/get-item/", bytes.NewBuffer(jsonPayload))
	fatalIfError(err, "Error creating request")

	req.Header.Set("Content-Type", "application/json")

	apiKey, err := getAPIKey()
	fatalIfError(err, "Error reading .api-key file")

	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	fatalIfError(err, "Error making request")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	fatalIfError(err, "Error reading response")

	var jsonResponse interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		log.Fatalf("Error parsing JSON response: %v\nRaw response: %s", err, string(body))
	}

	// process response

	prettyJSON, err := json.MarshalIndent(jsonResponse, "", "  ")
	if err != nil {
		log.Fatalf("Error formatting JSON: %v\nRaw response: %s", err, string(body))
	}

	fmt.Printf("%s\n", prettyJSON)
}
