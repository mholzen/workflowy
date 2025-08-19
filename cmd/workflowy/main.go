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
		printUsage()
		os.Exit(1)
	}

	verb := os.Args[1]

	switch verb {
	case "get":
		handleGet(os.Args[2:])
	case "list":
		handleList(os.Args[2:])
	case "tree":
		handleTree(os.Args[2:])
	default:
		fmt.Printf("Unknown command: %s\n\n", verb)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	progName := filepath.Base(os.Args[0])
	fmt.Printf("Usage: %s <command> [options]\n\n", progName)
	fmt.Println("Commands:")
	fmt.Printf("  %s get <item_id>                     Get details for a specific item\n", progName)
	fmt.Printf("  %s list <item_id> [--recursive]      List children of an item\n", progName)
	fmt.Printf("  %s tree <item_id>                    List children recursively (same as list --recursive)\n", progName)
	fmt.Println("\nUse 'None' as item_id to work with root items")
}

// Common helper functions
func requireItemID(args []string, command string) string {
	if len(args) < 1 {
		fmt.Printf("Error: %s command requires an item_id\n", command)
		printUsage()
		os.Exit(1)
	}
	return args[0]
}

func createClient() *workflowy.WorkflowyClient {
	return workflowy.NewWorkflowyClient(workflowy.WithAPIKeyFromFile(".api-key"))
}

func printJSON(response interface{}) {
	prettyJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		log.Fatalf("Error formatting JSON: %v", err)
	}
	fmt.Printf("%s\n", prettyJSON)
}

func handleGet(args []string) {
	itemID := requireItemID(args, "get")
	client := createClient()

	ctx := context.Background()
	response, err := client.GetItem(ctx, itemID)
	if err != nil {
		log.Fatalf("Error getting item: %v", err)
	}

	printJSON(response)
}

func handleList(args []string) {
	itemID := requireItemID(args, "list")
	recursive := len(args) > 1 && args[1] == "--recursive"
	client := createClient()

	ctx := context.Background()
	var response *workflowy.ListChildrenResponse
	var err error

	if recursive {
		fmt.Printf("Fetching children recursively for item: %s\n", itemID)
		response, err = client.ListChildrenRecursive(ctx, itemID)
	} else {
		fmt.Printf("Fetching direct children for item: %s\n", itemID)
		response, err = client.ListChildren(ctx, itemID)
	}

	if err != nil {
		log.Fatalf("Error listing children: %v", err)
	}

	printJSON(response)
}

func handleTree(args []string) {
	itemID := requireItemID(args, "tree")
	client := createClient()

	ctx := context.Background()
	fmt.Printf("Fetching complete tree for item: %s\n", itemID)
	response, err := client.ListChildrenRecursive(ctx, itemID)
	if err != nil {
		log.Fatalf("Error fetching tree: %v", err)
	}

	printJSON(response)
}
