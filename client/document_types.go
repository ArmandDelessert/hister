// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import "fmt"

// Document contains the JSON fields exchanged with the document and search APIs.
type Document struct {
	DocumentID         string         `json:"id,omitempty"`
	URL                string         `json:"url"`
	Domain             string         `json:"domain"`
	HTML               string         `json:"html"`
	HTMLKey            string         `json:"html_key"`
	Title              string         `json:"title"`
	Text               string         `json:"text"`
	Favicon            string         `json:"favicon"`
	FaviconKey         string         `json:"favicon_key"`
	Score              float64        `json:"score"`
	Added              int64          `json:"added"`
	Updated            int64          `json:"updated"`
	Type               DocType        `json:"type"`
	Language           string         `json:"language"`
	UserID             uint           `json:"user_id"`
	Label              string         `json:"label"`
	AddCount           uint           `json:"add_count"`
	Metadata           map[string]any `json:"metadata"`
	SkipSensitiveCheck bool           `json:"skip_sensitive_check"`
	Processed          bool           `json:"processed"`
}

// DocType represents the type of an indexed document.
type DocType int

const (
	Web DocType = iota
	Local
	RemoteFile
)

// String returns the human readable name of the DocType.
func (t DocType) String() string {
	switch t {
	case Web:
		return "web"
	case Local:
		return "local"
	case RemoteFile:
		return "remote"
	default:
		return "unknown"
	}
}

// ID returns the document identity within its owner scope.
func (d *Document) ID() string {
	if d.UserID != 0 {
		return fmt.Sprintf("%d:%s", d.UserID, d.URL)
	}
	return d.URL
}
