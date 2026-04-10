// Package extractor provides HTML content extraction for documents.
package extractor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/extractor/extractors/godoc"
	"github.com/asciimoo/hister/server/extractor/extractors/stackoverflow"
	"github.com/asciimoo/hister/server/extractor/extractors/ytdlp"
	"github.com/asciimoo/hister/server/types"
)

// Extractor extracts content from a Document.
type Extractor interface {
	// Name returns a human-readable identifier for the extractor.
	Name() string

	// Match reports whether this extractor is applicable to the given document.
	// Extract and Preview will only be called when Match returns true.
	Match(*document.Document) bool

	// Extract rewrites documents before the documents are added to the index.
	// The returned ExtractorState signals how the chain should proceed:
	// ExtractorStop means success, ExtractorContinue means try the next extractor,
	// ExtractorAbort means stop immediately and return the error.
	Extract(*document.Document) (types.ExtractorState, error)

	// Preview returns a rendered representation of the document suitable for
	// display (e.g. readable HTML or plain text).
	// The returned ExtractorState signals how the chain should proceed:
	// ExtractorStop means success, ExtractorContinue means try the next extractor,
	// ExtractorAbort means stop immediately and return the error.
	Preview(*document.Document) (types.PreviewResponse, types.ExtractorState, error)

	// GetConfig returns the extractor's current configuration. Before
	// SetConfig is called, implementations must return their default config.
	GetConfig() *config.Extractor

	// SetConfig applies cfg to the extractor, overwriting defaults.
	// Implementations should return an error for unrecognised option keys.
	SetConfig(*config.Extractor) error
}

// ErrNoExtractor is returned when no extractor can handle the document.
var ErrNoExtractor = errors.New("no extractor found")

var extractors = []Extractor{
	&stackoverflow.StackoverflowExtractor{},
	&godoc.GoDocExtractor{},
	&ytdlp.YtdlpExtractor{},
	&readabilityExtractor{},
	&defaultExtractor{},
}

// Init applies user-supplied extractor configurations on top of each
// extractor's defaults. It must be called before Extract or Preview.
// cfgs is keyed by lowercased extractor name (as Viper lowercases YAML keys).
func Init(cfgs map[string]*config.Extractor) error {
	for _, e := range extractors {
		def := e.GetConfig()
		merged := &config.Extractor{
			Enable:  def.Enable,
			Options: make(map[string]any, len(def.Options)),
		}
		maps.Copy(merged.Options, def.Options)
		if user, ok := cfgs[strings.ToLower(e.Name())]; ok && user != nil {
			merged.Enable = user.Enable
			maps.Copy(merged.Options, user.Options)
		}
		if err := e.SetConfig(merged); err != nil {
			return fmt.Errorf("extractor %s: %w", e.Name(), err)
		}
	}
	return nil
}

// Extract tries each registered extractor in order and returns the first
// successful result. Returns ErrNoExtractor if none succeed.
func Extract(d *document.Document) error {
	for _, e := range extractors {
		if !e.GetConfig().Enable {
			continue
		}
		if e.Match(d) {
			state, err := e.Extract(d)
			log.Debug().Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Extracting data")
			switch state {
			case types.ExtractorStop:
				return nil
			case types.ExtractorAbort:
				return fmt.Errorf("extractor %s: %w", e.Name(), err)
			default:
				if err != nil {
					log.Warn().Err(err).Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Failed to extract content")
				}
			}
		}
	}
	return ErrNoExtractor
}

// Preview returns a rendered preview of the document using the first matching
// extractor. Returns ErrNoExtractor if none match.
func Preview(d *document.Document) (types.PreviewResponse, error) {
	for _, e := range extractors {
		if !e.GetConfig().Enable {
			continue
		}
		if e.Match(d) {
			log.Debug().Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Creating preview")
			resp, state, err := e.Preview(d)
			switch state {
			case types.ExtractorStop:
				return resp, nil
			case types.ExtractorAbort:
				return types.PreviewResponse{}, fmt.Errorf("extractor %s: %w", e.Name(), err)
			default:
				if err != nil {
					log.Warn().Err(err).Str("URL", d.URL).Str("Extractor", e.Name()).Msg("Failed to preview content")
				}
			}
		}
	}
	return types.PreviewResponse{}, ErrNoExtractor
}

type defaultExtractor struct {
	cfg *config.Extractor
}

type readabilityExtractor struct {
	cfg *config.Extractor
}

func (e *defaultExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *defaultExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *readabilityExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

func (e *readabilityExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *defaultExtractor) Name() string {
	return "Default"
}

func (e *defaultExtractor) Match(_ *document.Document) bool {
	return true
}

func (e *defaultExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	d.Title = ""
	r := bytes.NewReader([]byte(d.HTML))
	doc := html.NewTokenizer(r)
	inBody := false
	skip := false
	var text strings.Builder
	var currentTag string
out:
	for {
		tt := doc.Next()
		switch tt {
		case html.ErrorToken:
			err := doc.Err()
			if errors.Is(err, io.EOF) {
				break out
			}
			return types.ExtractorStop, errors.New("failed to parse html: " + err.Error())
		case html.SelfClosingTagToken, html.StartTagToken:
			tn, _ := doc.TagName()
			currentTag = string(tn)
			switch currentTag {
			case "body":
				inBody = true
			case "script", "style", "noscript":
				skip = true
			}
		case html.TextToken:
			if currentTag == "title" {
				d.Title += strings.TrimSpace(string(doc.Text()))
			}
			if inBody && !skip {
				text.Write(doc.Text())
			}
		case html.EndTagToken:
			tn, _ := doc.TagName()
			switch string(tn) {
			case "body":
				inBody = false
			case "script", "style", "noscript":
				skip = false
			}
		}
	}
	d.Text = strings.TrimSpace(text.String())
	if d.Text == "" && d.Title == "" {
		return types.ExtractorStop, errors.New("no content found")
	}
	return types.ExtractorStop, nil
}

func (e *defaultExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	return types.PreviewResponse{Content: d.Text}, types.ExtractorStop, nil
}

func (e *readabilityExtractor) Name() string {
	return "Readability"
}

func (e *readabilityExtractor) Match(_ *document.Document) bool {
	return true
}

func (e *readabilityExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	r := bytes.NewReader([]byte(d.HTML))

	u, err := url.Parse(d.URL)
	if err != nil {
		return types.ExtractorStop, err
	}
	a, err := readability.FromReader(r, u)
	if err != nil {
		return types.ExtractorContinue, err
	}
	buf := bytes.NewBuffer(nil)
	if err := a.RenderText(buf); err != nil {
		return types.ExtractorContinue, err
	}
	d.Text = buf.String()
	d.Title = a.Title()
	d.SetFaviconURL(a.Favicon())
	return types.ExtractorStop, nil
}

func (e *readabilityExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	r := bytes.NewReader([]byte(d.HTML))
	u, err := url.Parse(d.URL)
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorStop, err
	}
	a, err := readability.FromReader(r, u)
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}
	var htmlContent strings.Builder
	if err := a.RenderHTML(&htmlContent); err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}
	return types.PreviewResponse{Content: htmlContent.String()}, types.ExtractorStop, nil
}
