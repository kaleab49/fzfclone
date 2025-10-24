package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	screen.Clear()
	drawText(screen, 0, 0, "🔍 Scanning files... please wait.", tcell.StyleDefault)
	screen.Show()

	// Channel to receive file paths
	fileCh := make(chan string, 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Concurrent file scanning
	go func() {
		defer close(fileCh)
		scanFiles(ctx, ".", fileCh)
	}()

	// Collect files
	var allFiles []string
	for path := range fileCh {
		allFiles = append(allFiles, path)
	}

	screen.Clear()
	drawText(screen, 0, 0, "✅ File scan complete!", tcell.StyleDefault)
	drawText(screen, 0, 2, "Type to search (press ESC to quit):", tcell.StyleDefault)
	screen.Show()

	input := ""
	for {
		ev := screen.PollEvent()
		switch ev := ev.(type) {
		case *tcell.EventKey:
			if ev.Key() == tcell.KeyEscape {
				return
			} else if ev.Key() == tcell.KeyBackspace || ev.Key() == tcell.KeyBackspace2 {
				if len(input) > 0 {
					input = input[:len(input)-1]
				}
			} else if ev.Key() == tcell.KeyEnter {
				// do nothing for now
			} else {
				input += string(ev.Rune())
			}

			results := fuzzy.Find(input, allFiles)
			showResults(screen, input, results)
		}
	}
}

// Walk files concurrently using errgroup
func scanFiles(ctx context.Context, root string, out chan<- string) {
	var g errgroup.Group
	var mu sync.Mutex

	g.Go(func() error {
		return godirwalk.Walk(root, &godirwalk.Options{
			Callback: func(path string, de *godirwalk.Dirent) error {
				if !de.IsDir() {
					mu.Lock()
					out <- path
					mu.Unlock()
				}
				return nil
			},
			Unsorted: true,
		})
	})

	_ = g.Wait()
}

func drawText(s tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, r := range text {
		s.SetContent(x+i, y, r, nil, style)
	}
}

func showResults(s tcell.Screen, query string, results []string) {
	s.Clear()
	drawText(s, 0, 0, fmt.Sprintf("Search: %s", query), tcell.StyleDefault)
	drawText(s, 0, 2, "Results:", tcell.StyleDefault)

	max := 15
	for i, r := range results {
		if i >= max {
			break
		}
		drawText(s, 0, 3+i, r, tcell.StyleDefault)
	}

	s.Show()
}
