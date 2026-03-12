// SPDX-FileContributor: slowerloris <taylor@teukka.tech>
//
// SPDX-License-Identifier: AGPL-3.0-or-later
package files

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

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

// Debounce so we don't spam the index as write events can file multiple times before closing a file after editing
const debounceTime = 200 * time.Millisecond

func WatchDirectories(dirs []string, callback func(string)) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error().Err(err).Msg("Failed to start file watcher")
	}
	defer watcher.Close()

	go func() {
		log.Debug().Msg("Starting file watcher")
		debounced := make(map[string]time.Timer)
		go func() {
			for path, timer := range maps.All(debounced) {
				<-timer.C
				callback(path)
			}
		}()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) {
					if debounceTimer, ok := debounced[event.Name]; ok {
						debounceTimer.Reset(debounceTime)
					} else {
						debounced[event.Name] = *time.NewTimer(debounceTime)
					}
				}
				if event.Has(fsnotify.Create) {
					st, err := os.Stat(event.Name)
					if err == nil {
						if st.IsDir() && !slices.Contains(watcher.WatchList(), event.Name) {
							watcher.Add(event.Name)
						} else {
							callback(event.Name)
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Error().Err(err).Msg("Watcher failed to process event")
			}
		}
	}()
	for _, dir := range dirs {
		dir = ExpandHome(dir)
		err = watcher.Add(dir)
		if err != nil {
			log.Error().Err(err).Str("path", dir).Msg("Failed to add path to file watcher")
		}
		err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				watcher.Add(path)
			}
			return nil
		})
	}
	<-make(chan struct{})
}
