package model_test

import (
	"testing"

	"github.com/asciimoo/hister/server/model"
	"github.com/asciimoo/hister/server/testutil"
)

func TestAnalyzerFingerprintPersistence(t *testing.T) {
	testutil.InitModel(t)
	if err := model.SetIndexerVersion(8); err != nil {
		t.Fatal(err)
	}
	if err := model.SetAnalyzerFingerprint("first"); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := model.GetAnalyzerFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint != "first" {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, "first")
	}
	if err := model.SetAnalyzerFingerprint("second"); err != nil {
		t.Fatal(err)
	}
	fingerprint, err = model.GetAnalyzerFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if fingerprint != "second" {
		t.Fatalf("fingerprint = %q, want %q", fingerprint, "second")
	}
	version, err := model.GetIndexerVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != 8 {
		t.Fatalf("indexer version = %d, want 8", version)
	}
}
