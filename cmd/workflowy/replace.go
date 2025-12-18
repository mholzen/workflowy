package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type ReplaceResult struct {
	ID         string `json:"id"`
	OldName    string `json:"old_name"`
	NewName    string `json:"new_name"`
	URL        string `json:"url"`
	Applied    bool   `json:"applied"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
}

func (rr ReplaceResult) String() string {
	if rr.Skipped {
		return fmt.Sprintf("%s: \"%s\" (skipped: %s)", rr.ID, rr.OldName, rr.SkipReason)
	}
	status := "→"
	if !rr.Applied {
		status = "→ (dry-run)"
	}
	return fmt.Sprintf("%s: \"%s\" %s \"%s\"", rr.ID, rr.OldName, status, rr.NewName)
}

type ReplaceOptions struct {
	Pattern     *regexp.Regexp
	Replacement string
	Interactive bool
	DryRun      bool
	Depth       int
}

func collectReplacements(items []*workflowy.Item, opts ReplaceOptions, currentDepth int, results *[]ReplaceResult) {
	if opts.Depth >= 0 && currentDepth > opts.Depth {
		return
	}

	for _, item := range items {
		if opts.Pattern.MatchString(item.Name) {
			newName := opts.Pattern.ReplaceAllString(item.Name, opts.Replacement)
			if newName != item.Name {
				*results = append(*results, ReplaceResult{
					ID:      item.ID,
					OldName: item.Name,
					NewName: newName,
					URL:     fmt.Sprintf("https://workflowy.com/#/%s", item.ID),
					Applied: false,
					Skipped: false,
				})
			}
		}

		if len(item.Children) > 0 {
			collectReplacements(item.Children, opts, currentDepth+1, results)
		}
	}
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
