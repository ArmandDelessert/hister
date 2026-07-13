package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/files"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/model"
)

var (
	ErrEmptyFile    = errors.New("empty file")
	ErrBinaryFile   = errors.New("binary file")
	ErrFileTooLarge = errors.New("file too large")

	maxFileSize int64 = 1024 * 1024 // 1MB default
)

type fileIndexOp int

const (
	fileIndexAdd fileIndexOp = iota
	fileIndexDelete
)

type fileIndexQueueItem struct {
	op     fileIndexOp
	path   string
	userID uint
}

type FileIndexQueue struct {
	mu      sync.Mutex
	pending map[string]fileIndexQueueItem
	notify  chan struct{}
}

func NewFileIndexQueue() *FileIndexQueue {
	return &FileIndexQueue{
		pending: make(map[string]fileIndexQueueItem),
		notify:  make(chan struct{}, 1),
	}
}

func (q *FileIndexQueue) EnqueueIndex(path string, userID uint) {
	q.enqueue(fileIndexQueueItem{
		op:     fileIndexAdd,
		path:   path,
		userID: userID,
	})
}

func (q *FileIndexQueue) EnqueueDelete(path string) {
	q.enqueue(fileIndexQueueItem{
		op:   fileIndexDelete,
		path: path,
	})
}

func (q *FileIndexQueue) enqueue(item fileIndexQueueItem) {
	q.mu.Lock()
	q.pending[item.path] = item
	q.mu.Unlock()

	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *FileIndexQueue) Run(ctx context.Context) error {
	for {
		if item, ok := q.pop(); ok {
			q.process(item)
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-q.notify:
		}
	}
}

func (q *FileIndexQueue) pop() (fileIndexQueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for path, item := range q.pending {
		delete(q.pending, path)
		return item, true
	}
	return fileIndexQueueItem{}, false
}

func (q *FileIndexQueue) process(item fileIndexQueueItem) {
	switch item.op {
	case fileIndexAdd:
		if err := IndexFile(item.path, item.userID); err != nil {
			log.Debug().Err(err).Str("path", item.path).Msg("Failed to index file")
		}
	case fileIndexDelete:
		if err := DeleteFile(item.path); err != nil {
			log.Debug().Err(err).Str("path", item.path).Msg("Failed to delete file from index")
		}
	}
}

func IndexAll(dirs []*config.Directory) {
	for _, dir := range dirs {
		expanded := files.ExpandHome(dir.Path)
		if err := indexDirectory(expanded, dir); err != nil {
			log.Error().Err(err).Str("directory", expanded).Msg("Failed to index directory")
		}
	}
}

func (q *FileIndexQueue) EnqueueAll(dirs []*config.Directory) {
	for _, dir := range dirs {
		expanded := files.ExpandHome(dir.Path)
		if err := q.enqueueDirectory(expanded, dir); err != nil {
			log.Error().Err(err).Str("directory", expanded).Msg("Failed to queue directory indexing")
		}
	}
}

func indexDirectory(dir string, cfg *config.Directory) error {
	log.Debug().Str("directory", dir).Msg("Indexing directory")

	indexed, skipped, err := walkDirectoryFiles(dir, cfg, func(path string, userID uint) bool {
		if err := IndexFile(path, userID); err != nil {
			log.Debug().Err(err).Str("path", path).Msg("Skipping file")
			return false
		}
		return true
	})

	log.Debug().Str("directory", dir).Int("indexed", indexed).Int("skipped", skipped).Msg("Directory indexing complete")
	return err
}

func (q *FileIndexQueue) enqueueDirectory(dir string, cfg *config.Directory) error {
	log.Debug().Str("directory", dir).Msg("Queueing directory indexing")

	queued, _, err := walkDirectoryFiles(dir, cfg, func(path string, userID uint) bool {
		q.EnqueueIndex(path, userID)
		return true
	})

	log.Debug().Str("directory", dir).Int("queued", queued).Msg("Directory indexing queued")
	return err
}

func walkDirectoryFiles(dir string, cfg *config.Directory, callback func(path string, userID uint) bool) (int, int, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return 0, 0, fmt.Errorf("not a directory: %s", dir)
	}

	var userID uint
	if cfg.User != "" {
		u, err := model.GetUser(cfg.User)
		if err != nil {
			log.Error().Err(err).Str("directory", dir).Msg("Failed to resolve user for directory")
			return 0, 0, fmt.Errorf("user %q not found for directory %s: %w", cfg.User, dir, err)
		}
		userID = u.ID
	}

	processed := 0
	skipped := 0

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			log.Warn().Err(err).Str("path", path).Msg("Error accessing path")
			return nil
		}
		if d.IsDir() {
			if path != dir && files.ShouldSkipDir(d.Name(), cfg.Excludes, cfg.IncludeHidden) {
				return filepath.SkipDir
			}
			return nil
		}
		if !cfg.IsMatching(d.Name()) {
			return nil
		}
		if callback(path, userID) {
			processed++
		} else {
			skipped++
		}
		return nil
	})

	return processed, skipped, err
}

func IndexFile(path string, userID uint) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return ErrEmptyFile
	}

	if info.Size() > maxFileSize {
		return fmt.Errorf("%w: %d bytes", ErrFileTooLarge, info.Size())
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	fileURL := files.PathToFileURL(absPath)

	// Skip if already indexed with the same modification time
	existing := GetByURLAndUser(fileURL, userID)
	if existing != nil && existing.Updated == info.ModTime().Unix() {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return &document.ReadFileError{
			Msg: err.Error(),
		}
	}

	doc := &document.Document{
		URL:     fileURL,
		Updated: info.ModTime().Unix(),
		UserID:  userID,
	}

	return indexFileContent(path, doc, content)
}

// DeleteFile removes the document for the given filesystem path from the index.
// It uses a url: field query so it removes the file across all users and
// language-specific sub-indexes. Returns nil if the document is not found.
func DeleteFile(path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}
	fileURL := files.PathToFileURL(absPath)
	_, err = DeleteByQuery("url:"+fileURL, nil, nil)
	return err
}
