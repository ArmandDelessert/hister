// SPDX-FileContributor: slowerloris <taylor@teukka.tech>
//
// SPDX-License-Identifier: AGPL-3.0-or-later
package files

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/asciimoo/hister/config"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// MatchesFilters reports whether a filename passes the given filetype, pattern, and exclude filters.
func MatchesFilters(name string, filetypes, patterns, excludes []string) bool {
	if len(excludes) > 0 {
		for _, pattern := range excludes {
			if matched, _ := filepath.Match(pattern, name); matched {
				return false
			}
		}
	}
	if len(filetypes) > 0 {
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		if !slices.Contains(filetypes, ext) {
			return false
		}
	}
	if len(patterns) > 0 {
		for _, pattern := range patterns {
			if matched, _ := filepath.Match(pattern, name); matched {
				return true
			}
		}
		return false
	}
	return true
}

// Debounce so we don't spam the index as write events can file multiple times before closing a file after editing
const debounceTime = 200 * time.Millisecond

// findMatchingDir returns the Directory config whose expanded path contains filePath, or nil.
func findMatchingDir(dirs []config.Directory, filePath string) *config.Directory {
	for i := range dirs {
		dirPath := filepath.Clean(ExpandHome(dirs[i].Path))
		if strings.HasPrefix(filePath, dirPath+"/") || filePath == dirPath {
			return &dirs[i]
		}
	}
	return nil
}

func WatchDirectories(ctx context.Context, dirs []config.Directory, callback func(string)) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	defer func() {
		if err := watcher.Close(); err != nil {
			log.Error().Err(err).Msg("Failed to close file watcher")
		}
	}()

	var mu sync.Mutex
	debounced := make(map[string]*time.Timer)

	log.Debug().Msg("Starting file watcher")

	// Add configured directories and their subdirectories to the watcher
	for _, dir := range dirs {
		expanded := ExpandHome(dir.Path)
		if err := watcher.Add(expanded); err != nil {
			log.Error().Err(err).Str("path", expanded).Msg("Failed to add path to file watcher")
		}
		_ = filepath.WalkDir(expanded, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				log.Warn().Err(err).Str("path", path).Msg("Error walking directory")
				return nil
			}
			if d.IsDir() {
				if err := watcher.Add(path); err != nil {
					log.Warn().Err(err).Str("path", path).Msg("Failed to watch subdirectory")
				}
			}
			return nil
		})
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Has(fsnotify.Write) {
				if dir := findMatchingDir(dirs, event.Name); dir != nil {
					if MatchesFilters(filepath.Base(event.Name), dir.Filetypes, dir.Patterns, dir.Excludes) {
						name := event.Name
						mu.Lock()
						if t, ok := debounced[name]; ok {
							t.Reset(debounceTime)
						} else {
							debounced[name] = time.AfterFunc(debounceTime, func() {
								mu.Lock()
								delete(debounced, name)
								mu.Unlock()
								callback(name)
							})
						}
						mu.Unlock()
					}
				}
			}
			if event.Has(fsnotify.Create) {
				st, err := os.Stat(event.Name)
				if err == nil {
					if st.IsDir() && !slices.Contains(watcher.WatchList(), event.Name) {
						if err := watcher.Add(event.Name); err != nil {
							log.Warn().Err(err).Str("path", event.Name).Msg("Failed to watch new directory")
						}
					} else if dir := findMatchingDir(dirs, event.Name); dir != nil {
						if MatchesFilters(filepath.Base(event.Name), dir.Filetypes, dir.Patterns, dir.Excludes) {
							callback(event.Name)
						}
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error().Err(err).Msg("Watcher failed to process event")
		}
	}
}
