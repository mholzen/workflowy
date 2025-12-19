package replace

import (
	"fmt"
	"regexp"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type Result struct {
	ID         string `json:"id"`
	OldName    string `json:"old_name"`
	NewName    string `json:"new_name"`
	URL        string `json:"url"`
	Applied    bool   `json:"applied"`
	Skipped    bool   `json:"skipped,omitempty"`
	SkipReason string `json:"skip_reason,omitempty"`
}

func (r Result) String() string {
	if r.Skipped {
		return fmt.Sprintf("%s: \"%s\" (skipped: %s)", r.ID, r.OldName, r.SkipReason)
	}
	status := "→"
	if !r.Applied {
		status = "→ (dry-run)"
	}
	return fmt.Sprintf("%s: \"%s\" %s \"%s\"", r.ID, r.OldName, status, r.NewName)
}

type Options struct {
	Pattern     *regexp.Regexp
	Replacement string
	Interactive bool
	DryRun      bool
	Depth       int
}

func CollectReplacements(items []*workflowy.Item, opts Options, currentDepth int, results *[]Result) {
	if opts.Depth >= 0 && currentDepth > opts.Depth {
		return
	}

	for _, item := range items {
		if opts.Pattern.MatchString(item.Name) {
			newName := opts.Pattern.ReplaceAllString(item.Name, opts.Replacement)
			if newName != item.Name {
				*results = append(*results, Result{
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
			CollectReplacements(item.Children, opts, currentDepth+1, results)
		}
	}
}
