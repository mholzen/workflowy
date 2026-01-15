package transform

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type Transformer func(string) (string, error)

var BuiltinTransformers = map[string]Transformer{
	"lowercase":      Lowercase,
	"uppercase":      Uppercase,
	"capitalize":     Capitalize,
	"title":          TitleCase,
	"trim":           Trim,
	"no-punctuation": RemovePunctuation,
	"no-whitespace":  RemoveWhitespace,
}

func ListBuiltins() []string {
	names := make([]string, 0, len(BuiltinTransformers))
	for name := range BuiltinTransformers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type Field int

const (
	FieldName Field = 1 << iota
	FieldNote
)

type Options struct {
	Transformer Transformer
	Fields      Field
	DryRun      bool
	Interactive bool
	Depth       int
	AsChild     bool
}

type Result struct {
	Item        *workflowy.Item `json:"-"`
	ID          string          `json:"id"`
	URL         string          `json:"url"`
	Field       string          `json:"field"`
	Original    string          `json:"original"`
	New         string          `json:"new"`
	Applied     bool            `json:"applied"`
	Skipped     bool            `json:"skipped,omitempty"`
	SkipReason  string          `json:"skip_reason,omitempty"`
	Error       error           `json:"error,omitempty"`
	CreatedID   string          `json:"created_id,omitempty"`
}

func (r Result) String() string {
	if r.Skipped {
		return r.ID + " (" + r.Field + "): \"" + r.Original + "\" (skipped: " + r.SkipReason + ")"
	}
	status := "→"
	if !r.Applied {
		status = "→ (dry-run)"
	}
	result := r.ID + " (" + r.Field + "): \"" + r.Original + "\" " + status + " \"" + r.New + "\""
	if r.CreatedID != "" {
		result += " [child: " + r.CreatedID + "]"
	}
	return result
}

func CollectTransformations(items []*workflowy.Item, opts Options, depth int, results *[]Result) {
	if opts.Depth >= 0 && depth > opts.Depth {
		return
	}

	for _, item := range items {
		if opts.Fields&FieldName != 0 {
			collectFieldTransformation(item, "name", item.Name, opts.Transformer, results)
		}

		if opts.Fields&FieldNote != 0 && item.Note != nil && *item.Note != "" {
			collectFieldTransformation(item, "note", *item.Note, opts.Transformer, results)
		}

		if len(item.Children) > 0 {
			CollectTransformations(item.Children, opts, depth+1, results)
		}
	}
}

func collectFieldTransformation(item *workflowy.Item, field, value string, t Transformer, results *[]Result) {
	transformed, err := t(value)
	if err != nil {
		*results = append(*results, Result{
			Item:       item,
			ID:         item.ID,
			URL:        "https://workflowy.com/#/" + item.ID,
			Field:      field,
			Original:   value,
			Error:      err,
			SkipReason: err.Error(),
			Skipped:    true,
		})
		return
	}

	if transformed == value {
		return
	}

	*results = append(*results, Result{
		Item:     item,
		ID:       item.ID,
		URL:      "https://workflowy.com/#/" + item.ID,
		Field:    field,
		Original: value,
		New:      transformed,
	})
}

func Lowercase(s string) (string, error) {
	return strings.ToLower(s), nil
}

func Uppercase(s string) (string, error) {
	return strings.ToUpper(s), nil
}

func Trim(s string) (string, error) {
	return strings.TrimSpace(s), nil
}

func Capitalize(s string) (string, error) {
	if len(s) == 0 {
		return s, nil
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes), nil
}

func TitleCase(s string) (string, error) {
	caser := cases.Title(language.English)
	return caser.String(s), nil
}

func RemovePunctuation(s string) (string, error) {
	return strings.Map(func(r rune) rune {
		if unicode.IsPunct(r) {
			return -1
		}
		return r
	}, s), nil
}

func RemoveWhitespace(s string) (string, error) {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s), nil
}

func ShellTransformer(cmdTemplate string) Transformer {
	return func(s string) (string, error) {
		cmd := strings.ReplaceAll(cmdTemplate, "{}", s)
		out, err := exec.Command("sh", "-c", cmd).Output()
		if err != nil {
			return "", err
		}
		return strings.TrimSuffix(string(out), "\n"), nil
	}
}

func ResolveTransformer(transformName, execCmd string) (Transformer, error) {
	if execCmd != "" && transformName != "" {
		return nil, fmt.Errorf("cannot specify both transform name and exec")
	}

	if execCmd != "" {
		return ShellTransformer(execCmd), nil
	}

	if transformName == "" {
		return nil, fmt.Errorf("transform name or exec is required")
	}

	t, ok := BuiltinTransformers[transformName]
	if !ok {
		return nil, fmt.Errorf("unknown transform: %s (available: %s)",
			transformName, strings.Join(ListBuiltins(), ", "))
	}
	return t, nil
}

func DetermineFields(name, note bool) Field {
	if !name && !note {
		return FieldName
	}

	var fields Field
	if name {
		fields |= FieldName
	}
	if note {
		fields |= FieldNote
	}
	return fields
}

func BuildUpdateRequest(result *Result) *workflowy.UpdateNodeRequest {
	req := &workflowy.UpdateNodeRequest{}
	if result.Field == "name" {
		req.Name = &result.New
	} else if result.Field == "note" {
		req.Note = &result.New
	}
	return req
}

type Applier interface {
	UpdateNode(ctx context.Context, itemID string, req *workflowy.UpdateNodeRequest) (*workflowy.UpdateNodeResponse, error)
	CreateNode(ctx context.Context, req *workflowy.CreateNodeRequest) (*workflowy.CreateNodeResponse, error)
}

func ApplyResults(ctx context.Context, client Applier, results []Result) {
	ApplyResultsWithOptions(ctx, client, results, false)
}

func ApplyResultsWithOptions(ctx context.Context, client Applier, results []Result, asChild bool) {
	for i := range results {
		result := &results[i]
		if result.Skipped {
			continue
		}

		if asChild {
			position := "top"
			req := &workflowy.CreateNodeRequest{
				ParentID: result.ID,
				Position: &position,
			}
			if result.Field == "name" {
				req.Name = result.New
			} else if result.Field == "note" {
				req.Note = &result.New
			}
			resp, err := client.CreateNode(ctx, req)
			if err != nil {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("create child failed: %v", err)
				continue
			}
			result.CreatedID = resp.ItemID
			result.Applied = true
		} else {
			req := BuildUpdateRequest(result)
			if _, err := client.UpdateNode(ctx, result.ID, req); err != nil {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("update failed: %v", err)
				continue
			}
			result.Applied = true
		}
	}
}

type SplitResult struct {
	ParentID   string   `json:"parent_id"`
	ParentURL  string   `json:"parent_url"`
	Original   string   `json:"original"`
	Parts      []string `json:"parts"`
	CreatedIDs []string `json:"created_ids,omitempty"`
	Applied    bool     `json:"applied"`
	Skipped    bool     `json:"skipped,omitempty"`
	SkipReason string   `json:"skip_reason,omitempty"`
}

func (r SplitResult) String() string {
	if r.Skipped {
		return fmt.Sprintf("%s: \"%s\" (skipped: %s)", r.ParentID, r.Original, r.SkipReason)
	}
	status := "→"
	if !r.Applied {
		status = "→ (dry-run)"
	}
	return fmt.Sprintf("%s: \"%s\" %s %d children", r.ParentID, r.Original, status, len(r.Parts))
}

func UnescapeSeparator(s string) string {
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\r", "\r")
	return s
}

func Split(text, separator string, skipEmpty bool) []string {
	parts := strings.Split(text, separator)
	if !skipEmpty {
		return parts
	}

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func CollectSplits(items []*workflowy.Item, separator string, field Field, skipEmpty bool, depth int, maxDepth int, results *[]SplitResult) {
	if maxDepth >= 0 && depth > maxDepth {
		return
	}

	for _, item := range items {
		var text string
		if field&FieldName != 0 {
			text = item.Name
		} else if field&FieldNote != 0 && item.Note != nil {
			text = *item.Note
		}

		if text != "" {
			parts := Split(text, separator, skipEmpty)
			if len(parts) > 1 {
				*results = append(*results, SplitResult{
					ParentID:  item.ID,
					ParentURL: "https://workflowy.com/#/" + item.ID,
					Original:  text,
					Parts:     parts,
				})
			}
		}

		if len(item.Children) > 0 {
			CollectSplits(item.Children, separator, field, skipEmpty, depth+1, maxDepth, results)
		}
	}
}

func ApplySplitResults(ctx context.Context, client Applier, results []SplitResult) {
	for i := range results {
		result := &results[i]
		if result.Skipped {
			continue
		}

		createdIDs := make([]string, 0, len(result.Parts))
		for j := len(result.Parts) - 1; j >= 0; j-- {
			part := result.Parts[j]
			position := "top"
			req := &workflowy.CreateNodeRequest{
				ParentID: result.ParentID,
				Name:     part,
				Position: &position,
			}
			resp, err := client.CreateNode(ctx, req)
			if err != nil {
				result.Skipped = true
				result.SkipReason = fmt.Sprintf("create failed for part %d: %v", j, err)
				break
			}
			createdIDs = append([]string{resp.ItemID}, createdIDs...)
		}

		if !result.Skipped {
			result.CreatedIDs = createdIDs
			result.Applied = true
		}
	}
}
