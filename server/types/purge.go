package types

import (
	"errors"
	"time"
)

// PurgeRequest selects retained SQL data independently of current documents.
type PurgeRequest struct {
	Before    time.Time `json:"before"`
	Versions  bool      `json:"versions,omitempty"`
	History   bool      `json:"history,omitempty"`
	CrawlJobs bool      `json:"crawl_jobs,omitempty"`
	DryRun    bool      `json:"dry_run,omitempty"`
}

func (r PurgeRequest) Validate() error {
	if !r.Versions && !r.History && !r.CrawlJobs {
		return errors.New("select versions, history, or crawl_jobs to purge")
	}
	if r.Before.IsZero() || r.Before.After(time.Now()) {
		return errors.New("before must be a nonzero timestamp in the past")
	}
	return nil
}

// PurgeResult counts matching rows for a dry run and removed rows otherwise.
type PurgeResult struct {
	DryRun         bool  `json:"dry_run"`
	Versions       int64 `json:"versions"`
	HistoryEntries int64 `json:"history_entries"`
	HistoryQueries int64 `json:"history_queries"`
	HistoryURLs    int64 `json:"history_urls"`
	CrawlJobs      int64 `json:"crawl_jobs"`
	CrawlURLs      int64 `json:"crawl_urls"`
}
