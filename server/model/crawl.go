// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"gorm.io/gorm"
)

// CrawlJobStatus values.
const (
	CrawlJobRunning     = "running"
	CrawlJobCompleted   = "completed"
	CrawlJobInterrupted = "interrupted"
)

// CrawlURLStatus values.
const (
	CrawlURLPending    = "pending"
	CrawlURLInProgress = "in_progress"
	CrawlURLDone       = "done"
	CrawlURLFailed     = "failed"
	CrawlURLSkipped    = "skipped"
)

// CrawlJob stores the configuration and status of a persistent crawl job.
type CrawlJob struct {
	ID             string    `gorm:"primaryKey" json:"id"`
	StartURL       string    `json:"start_url"`
	ValidatorRules string    `gorm:"type:text" json:"validator_rules"` // JSON-encoded ValidatorRules
	Label          string    `json:"label"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CrawlURL tracks every URL discovered during a crawl job.
type CrawlURL struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	JobID     string    `gorm:"uniqueIndex:idx_crawl_job_url;not null" json:"job_id"`
	URL       string    `gorm:"uniqueIndex:idx_crawl_job_url;not null" json:"url"`
	Depth     int       `json:"depth"`
	Status    string    `gorm:"not null;default:pending" json:"status"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GenerateCrawlJobID returns a random 8-character hex string suitable as a job ID.
func GenerateCrawlJobID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateCrawlJob inserts a new CrawlJob record.
func CreateCrawlJob(id, startURL, validatorRules, label string) error {
	return DB.Create(&CrawlJob{
		ID:             id,
		StartURL:       startURL,
		ValidatorRules: validatorRules,
		Label:          label,
		Status:         CrawlJobRunning,
	}).Error
}

// GetCrawlJob returns the job with the given ID, or (nil, nil) when not found.
func GetCrawlJob(id string) (*CrawlJob, error) {
	var job CrawlJob
	err := DB.Where("id = ?", id).First(&job).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// UpdateCrawlJobStatus updates the status field of a job.
func UpdateCrawlJobStatus(id, status string) error {
	return DB.Model(&CrawlJob{}).Where("id = ?", id).Update("status", status).Error
}

// InsertCrawlURLIfNotExists adds a URL to the job's queue only when it has not
// been seen before (the unique index on job_id+url enforces this).
func InsertCrawlURLIfNotExists(jobID, rawURL string, depth int) error {
	cu := CrawlURL{JobID: jobID, URL: rawURL}
	result := DB.Where(cu).FirstOrCreate(&cu, CrawlURL{
		JobID:  jobID,
		URL:    rawURL,
		Depth:  depth,
		Status: CrawlURLPending,
	})
	return result.Error
}

// InsertCrawlURLDone records a URL as already done without going through the
// pending state (used for redirect targets that have been fetched indirectly).
func InsertCrawlURLDone(jobID, rawURL string, depth int) error {
	// Update if already present, otherwise insert as done.
	result := DB.Model(&CrawlURL{}).
		Where("job_id = ? AND url = ?", jobID, rawURL).
		Updates(map[string]any{"status": CrawlURLDone, "error": ""})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return DB.Create(&CrawlURL{
			JobID:  jobID,
			URL:    rawURL,
			Depth:  depth,
			Status: CrawlURLDone,
		}).Error
	}
	return nil
}

// NextPendingCrawlURL returns the oldest pending URL for the job.
// Returns (nil, nil) when no pending URLs remain.
func NextPendingCrawlURL(jobID string) (*CrawlURL, error) {
	var cu CrawlURL
	err := DB.Where("job_id = ? AND status = ?", jobID, CrawlURLPending).
		Order("id ASC").
		First(&cu).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &cu, nil
}

// UpdateCrawlURLStatus sets the status and optional error message on a URL row.
func UpdateCrawlURLStatus(id uint, status, errMsg string) error {
	return DB.Model(&CrawlURL{}).Where("id = ?", id).
		Updates(map[string]any{"status": status, "error": errMsg}).Error
}

// ResetInProgressCrawlURLs moves all in_progress URLs back to pending so they
// are retried after a crash or interruption.
func ResetInProgressCrawlURLs(jobID string) error {
	return DB.Model(&CrawlURL{}).
		Where("job_id = ? AND status = ?", jobID, CrawlURLInProgress).
		Update("status", CrawlURLPending).Error
}

// CountCrawlURLsByStatus returns the number of URLs with the given status for a job.
func CountCrawlURLsByStatus(jobID, status string) (int64, error) {
	var count int64
	err := DB.Model(&CrawlURL{}).
		Where("job_id = ? AND status = ?", jobID, status).
		Count(&count).Error
	return count, err
}

// ListCrawlJobs returns all crawl jobs ordered by creation time descending.
func ListCrawlJobs() ([]*CrawlJob, error) {
	var jobs []*CrawlJob
	err := DB.Order("created_at DESC").Find(&jobs).Error
	return jobs, err
}

// DeleteCrawlJob removes a job and all its associated URL rows.
func DeleteCrawlJob(id string) error {
	if err := DB.Where("job_id = ?", id).Delete(&CrawlURL{}).Error; err != nil {
		return err
	}
	return DB.Where("id = ?", id).Delete(&CrawlJob{}).Error
}

// CrawlJobStats contains aggregate counts for a job's URLs.
type CrawlJobStats struct {
	Pending    int64
	InProgress int64
	Done       int64
	Failed     int64
	Skipped    int64
}

// GetCrawlJobStats returns URL counts per status for the given job.
func GetCrawlJobStats(jobID string) (CrawlJobStats, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	err := DB.Model(&CrawlURL{}).
		Select("status, count(*) as count").
		Where("job_id = ?", jobID).
		Group("status").
		Scan(&rows).Error
	if err != nil {
		return CrawlJobStats{}, err
	}
	var s CrawlJobStats
	for _, r := range rows {
		switch r.Status {
		case CrawlURLPending:
			s.Pending = r.Count
		case CrawlURLInProgress:
			s.InProgress = r.Count
		case CrawlURLDone:
			s.Done = r.Count
		case CrawlURLFailed:
			s.Failed = r.Count
		case CrawlURLSkipped:
			s.Skipped = r.Count
		}
	}
	return s, nil
}
