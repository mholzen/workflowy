package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/mholzen/workflowy/pkg/replace"
	"github.com/mholzen/workflowy/pkg/workflowy"
)

type ReplaceResult = replace.Result
type ReplaceOptions = replace.Options

func collectReplacements(items []*workflowy.Item, opts ReplaceOptions, currentDepth int, results *[]ReplaceResult) {
	replace.CollectReplacements(items, opts, currentDepth, results)
}

func promptConfirmation(result ReplaceResult) (confirm bool, quit bool) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Replace \"%s\" → \"%s\"? [y/N/q] ", result.OldName, result.NewName)

	response, err := reader.ReadString('\n')
	if err != nil {
		return false, false
	}

	response = strings.TrimSpace(strings.ToLower(response))
	if response == "q" || response == "quit" {
		return false, true
	}
	return response == "y" || response == "yes", false
}
