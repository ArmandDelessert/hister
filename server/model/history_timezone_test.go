package model_test

import (
	"testing"
	"time"

	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

// TestHistoryDateRangeIgnoresServerTimezone pins that a row written by a server
// running west of UTC is still found by a range query expressed in UTC. SQLite
// keeps whatever offset a time value carries and compares those values as text,
// so a timestamp stored as local wall clock sorts by that wall clock: an entry
// written at 20:21-07:00 reads as "2026-09-09 ..." and falls outside the UTC day
// it actually belongs to.
func TestHistoryDateRangeIgnoresServerTimezone(t *testing.T) {
	originalLocal := time.Local
	time.Local = time.FixedZone("UTC-8", -8*60*60)
	t.Cleanup(func() { time.Local = originalLocal })

	testutil.InitModel(t)
	if err := model.UpdateHistory(0, "q", "https://example.com", "Example"); err != nil {
		t.Fatalf("UpdateHistory() error: %v", err)
	}

	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	dayEnd := dayStart.AddDate(0, 0, 1)

	timestamps, err := model.GetHistoryItemTimestampsFilteredByDate(0, "", dayStart.Unix(), dayEnd.Unix())
	if err != nil {
		t.Fatalf("GetHistoryItemTimestampsFilteredByDate() error: %v", err)
	}
	if len(timestamps) != 1 {
		t.Fatalf("timestamps in the current UTC day = %d, want 1", len(timestamps))
	}

	if got := timestamps[0].UTC(); got.Before(dayStart) || !got.Before(dayEnd) {
		t.Errorf("stored timestamp %s is outside the queried day %s..%s", got, dayStart, dayEnd)
	}
}
