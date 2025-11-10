package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/karrick/godirwalk"
	"github.com/lithammer/fuzzysearch/fuzzy"
	"golang.org/x/sync/errgroup"
)

type FileResult struct {
	Path  string
	Score int
}

func main() {
	// CLI flags
	rootFlag := flag.String("root", ".", "Root directory to scan")
	maxResultsFlag := flag.Int("max-results", 15, "Maximum number of results to display")
	hiddenFlag := flag.Bool("hidden", false, "Include hidden files and directories")
	ignoreGitFlag := flag.Bool("ignore-git", true, "Respect .gitignore in the root directory if present")
	flag.Parse()

	screen, err := tcell.NewScreen()
	if err != nil {
		fmt.Println("Error creating screen:", err)
		os.Exit(1)
	}
	if err := screen.Init(); err != nil {
		fmt.Println("Error initializing screen:", err)
		os.Exit(1)
	}
	defer screen.Fini()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	screen.Clear()
	drawText(screen, 0, 0, "Scanning files... type to search (ESC to quit).", tcell.StyleDefault)
	screen.Show()

	fileCh := make(chan string, 1024)
	var filesMu sync.RWMutex
	var allFiles []string

	// Load ignore rules
	ignoredDirs, ignoredGlobs := loadIgnoreRules(*rootFlag, *ignoreGitFlag)
	defaultIgnored := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"dist":         {},
		"build":        {},
		"bin":          {},
		"vendor":       {},
	}
	for d := range defaultIgnored {
		if _, ok := ignoredDirs[d]; !ok {
			ignoredDirs[d] = struct{}{}
		}
	}

	go func() {
		defer close(fileCh)
		scanFiles(ctx, *rootFlag, fileCh, ignoredDirs, ignoredGlobs, *hiddenFlag)
	}()

	// Collector
	go func() {
		for path := range fileCh {
			filesMu.Lock()
			allFiles = append(allFiles, path)
			filesMu.Unlock()
		}
	}()

	input := ""
	lastResults := []string{}
	selected := 0
	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventResize:
			screen.Clear()
			showResults(screen, input, lastResults, *maxResultsFlag, selected)
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				cancel()
				return
			} else if ev.Key() == tcell.KeyUp {
				if selected > 0 {
					selected--
				}
				showResults(screen, input, lastResults, *maxResultsFlag, selected)
				continue
			} else if ev.Key() == tcell.KeyDown {
				if selected+1 < len(lastResults) && selected+1 < *maxResultsFlag {
					selected++
				}
				showResults(screen, input, lastResults, *maxResultsFlag, selected)
				continue
			} else if ev.Key() == tcell.KeyEnter {
				if len(lastResults) > 0 {
					fmt.Println(lastResults[selected])
				}
				cancel()
				return
			} else if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			} else {
				input += string(ev.Rune())
			}

			filesMu.RLock()
			candidates := make([]string, len(allFiles))
			copy(candidates, allFiles)
			filesMu.RUnlock()

			// Ranked fuzzy matching
			ranks := fuzzy.RankFindFold(input, candidates)
			// Sort by ascending distance, then shorter target first
			sort.Slice(ranks, func(i, j int) bool {
				if ranks[i].Distance == ranks[j].Distance {
					return len(ranks[i].Target) < len(ranks[j].Target)
				}
				return ranks[i].Distance < ranks[j].Distance
			})
			// Build top-N list
			limit := *maxResultsFlag
			if limit > len(ranks) {
				limit = len(ranks)
			}
			results := make([]string, 0, limit)
			for i := 0; i < limit; i++ {
				results = append(results, ranks[i].Target)
			}
			lastResults = results
			if selected >= len(lastResults) {
				selected = len(lastResults) - 1
				if selected < 0 {
					selected = 0
				}
			}
			showResults(screen, input, results, *maxResultsFlag, selected)
		}
	}
}

// Walk files concurrently using errgroup
func scanFiles(ctx context.Context, root string, out chan<- string, ignoredDirs map[string]struct{}, ignoredGlobs []string, includeHidden bool) {
	var g errgroup.Group

	g.Go(func() error {
		rootClean := filepath.Clean(root)
		return godirwalk.Walk(root, &godirwalk.Options{
			Unsorted:            true,
			FollowSymbolicLinks: false,
			Callback: func(path string, de *godirwalk.Dirent) error {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				base := filepath.Base(path)
				if de.IsDir() {
					// Do not treat the root directory as hidden even if it's "."
					if !includeHidden && strings.HasPrefix(base, ".") && filepath.Clean(path) != rootClean {
						return godirwalk.SkipThis
					}
					if _, skip := ignoredDirs[base]; skip {
						return godirwalk.SkipThis
					}
					for _, glob := range ignoredGlobs {
						// best-effort glob skip
						if ok, _ := filepath.Match(glob, base); ok {
							return godirwalk.SkipThis
						}
					}
					return nil
				}
				// file
				if !includeHidden && strings.HasPrefix(base, ".") {
					return nil
				}
				out <- path
				return nil
			},
		})
	})

	_ = g.Wait()
}

func drawText(s tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, r := range text {
		s.SetContent(x+i, y, r, nil, style)
	}
}

func showResults(s tcell.Screen, query string, results []string, max int, selected int) {
	s.Clear()
	drawText(s, 0, 0, fmt.Sprintf("Search: %s", query), tcell.StyleDefault)
	drawText(s, 0, 2, "Results:", tcell.StyleDefault)

	for i, r := range results {
		if i >= max {
			break
		}
		style := tcell.StyleDefault
		prefix := "  "
		if i == selected {
			style = style.Reverse(true)
			prefix = "> "
		}
		drawText(s, 0, 3+i, prefix+r, style)
	}

	// Footer / help
	w, h := s.Size()
	_ = w
	footer := "↑/↓ move  •  Enter select  •  ESC quit"
	drawText(s, 0, h-1, footer, tcell.StyleDefault.Dim(true))

	s.Show()
}

// loadIgnoreRules reads a .gitignore at root (if requested) and returns a set of directory names
// to skip and a list of simple glob patterns (best-effort).
func loadIgnoreRules(root string, respectGitignore bool) (map[string]struct{}, []string) {
	ignoredDirs := make(map[string]struct{})
	var globs []string
	if !respectGitignore {
		return ignoredDirs, globs
	}
	giPath := filepath.Join(root, ".gitignore")
	data, err := os.ReadFile(giPath)
	if err != nil {
		return ignoredDirs, globs
	}
	lines := strings.Split(string(data), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		// Normalize trailing slash meaning directory
		if strings.HasSuffix(ln, "/") {
			name := strings.TrimSuffix(ln, "/")
			if name != "" {
				ignoredDirs[name] = struct{}{}
			}
			continue
		}
		// Simple basename directory entry
		if !strings.ContainsAny(ln, "*?[]") && !strings.Contains(ln, "/") {
			ignoredDirs[ln] = struct{}{}
			continue
		}
		// Fallback to glob list (best-effort)
		globs = append(globs, ln)
	}
	return ignoredDirs, globs
}
