package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/daearol/jockv2/backend/internal/cache"
	"github.com/daearol/jockv2/backend/internal/dsl"
)

// Global state for REPL session reuse.
var replCtx *dsl.EvalContext

func main() {
	repoPath := detectRepoPath()

	if len(os.Args) > 1 {
		query := strings.Join(os.Args[1:], " ")
		runQuery(repoPath, query)
		return
	}

	// REPL mode — create shared cache and alias store for the session
	replCtx = &dsl.EvalContext{
		RepoPath: repoPath,
		DryRun:   false,
		Ctx:      context.Background(),
		Cache:    cache.NewManager(30 * time.Second),
		Aliases:  dsl.NewAliasStore(),
	}

	fmt.Println("jock query REPL (type 'exit' to quit)")
	fmt.Printf("repo: %s\n\n", repoPath)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("jock> ")
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "exit" || line == "quit" {
			break
		}
		if line == "" {
			fmt.Print("jock> ")
			continue
		}
		runQuery(repoPath, line)
		fmt.Print("\njock> ")
	}
	fmt.Println()
}

func runQuery(repoPath, query string) {
	// REPL mode uses RunQuery for alias support
	if replCtx != nil {
		result, err := dsl.RunQuery(replCtx, query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return
		}
		printResult(result)
		return
	}

	// Batch mode — no aliases
	pipeline, err := dsl.Parse(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %v\n", err)
		return
	}

	ctx := &dsl.EvalContext{
		RepoPath: repoPath,
		DryRun:   false,
		Ctx:      context.Background(),
	}
	result, err := dsl.Evaluate(ctx, pipeline)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}

	printResult(result)
}

func printResult(result *dsl.Result) {
	switch result.Kind {
	case "commits":
		printCommits(result.Commits)
	case "branches":
		printBranches(result.Branches)
	case "count":
		fmt.Printf("%d\n", result.Count)
	case "aggregate":
		printAggregates(result)
	case "files":
		printFiles(result.Files)
	case "formatted":
		fmt.Println(result.FormattedOutput)
	case "action_report":
		if result.Report != nil {
			printActionReport(result.Report)
		}
	}
}

func printCommits(commits []dsl.CommitResult) {
	if len(commits) == 0 {
		fmt.Println("(no commits)")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "HASH\tAUTHOR\tDATE\tMESSAGE")
	for _, c := range commits {
		hash := c.Hash
		if len(hash) > 7 {
			hash = hash[:7]
		}
		msg := c.Message
		if len(msg) > 60 {
			msg = msg[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", hash, c.Author, c.Date, msg)
	}
	w.Flush()
}

func printBranches(branches []dsl.BranchResult) {
	if len(branches) == 0 {
		fmt.Println("(no branches)")
		return
	}

	for _, b := range branches {
		prefix := "  "
		if b.IsCurrent {
			prefix = "* "
		}
		line := prefix + b.Name
		if b.Remote != "" {
			line += fmt.Sprintf(" -> %s", b.Remote)
		}
		fmt.Println(line)
	}
}

func printFiles(files []dsl.FileResult) {
	if len(files) == 0 {
		fmt.Println("(no files)")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PATH\tADDITIONS\tDELETIONS\tCOMMITS")
	for _, f := range files {
		fmt.Fprintf(w, "%s\t+%d\t-%d\t%d\n", f.Path, f.Additions, f.Deletions, f.Commits)
	}
	w.Flush()
}

func printAggregates(result *dsl.Result) {
	if len(result.Aggregates) == 1 && result.Aggregates[0].Group == "" {
		label := fmt.Sprintf("%s %s", result.AggFunc, result.AggField)
		fmt.Printf("%s: %.0f\n", strings.TrimSpace(label), result.Aggregates[0].Value)
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	header := strings.ToUpper(result.GroupField)
	if header == "" {
		header = "GROUP"
	}
	valHeader := strings.ToUpper(fmt.Sprintf("%s %s", result.AggFunc, result.AggField))
	fmt.Fprintf(w, "%s\t%s\n", header, strings.TrimSpace(valHeader))
	for _, row := range result.Aggregates {
		if result.AggFunc == "avg" {
			fmt.Fprintf(w, "%s\t%.1f\n", row.Group, row.Value)
		} else {
			fmt.Fprintf(w, "%s\t%.0f\n", row.Group, row.Value)
		}
	}
	w.Flush()
}

func printActionReport(report *dsl.ActionReport) {
	if report.DryRun {
		fmt.Println("[DRY RUN]")
	}

	fmt.Printf("action: %s\n", report.Action)
	fmt.Printf("result: %s\n", report.Description)

	if len(report.Affected) > 0 {
		fmt.Printf("affected: %d commit(s)\n", len(report.Affected))
		for _, h := range report.Affected {
			hash := h
			if len(hash) > 7 {
				hash = hash[:7]
			}
			fmt.Printf("  %s\n", hash)
		}
	}

	if len(report.Errors) > 0 {
		fmt.Println("\nerrors:")
		for _, e := range report.Errors {
			fmt.Printf("  %s\n", e)
		}
	}
}

func detectRepoPath() string {
	// Check -C flag
	for i, arg := range os.Args {
		if arg == "-C" && i+1 < len(os.Args) {
			return os.Args[i+1]
		}
	}

	// Check env var
	if p := os.Getenv("JOCK_REPO"); p != "" {
		return p
	}

	// Walk up from cwd looking for .git
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "."
}
