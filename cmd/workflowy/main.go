package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Printf("Usage: %s <item_id>\n", filepath.Base(os.Args[0]))
		os.Exit(1)
	}

	itemID := os.Args[1]

	client := workflowy.NewWorkflowyClient(workflowy.WithAPIKeyFromFile(".api-key"))

	// Get the item
	ctx := context.Background()
	response, err := client.GetItem(ctx, itemID)
	if err != nil {
		log.Fatalf("Error getting item: %v", err)
	}

	// Format and print the response
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Error formatting JSON: %v", err)
	}

	fmt.Printf("%s\n", prettyJSON)
}
