// Package stackoverflow provides an extractor for stackoverflow.com questions.
package stackoverflow

import (
	"fmt"
	"strings"

	"github.com/asciimoo/hister/config"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/sanitizer"
	"github.com/asciimoo/hister/server/types"

	"github.com/PuerkitoBio/goquery"
)

const matchURLPrefix = "https://stackoverflow.com/questions/"

type StackoverflowExtractor struct {
	cfg *config.Extractor
}

func (e *StackoverflowExtractor) Name() string {
	return "Stackoverflow"
}

// Description returns a short summary of what this extractor does.
func (e *StackoverflowExtractor) Description() string {
	return "Extracts the question body and all answers from Stack Overflow question pages."
}

// GetConfig returns the extractor's current configuration.
func (e *StackoverflowExtractor) GetConfig() *config.Extractor {
	if e.cfg == nil {
		return &config.Extractor{Enable: true, Options: map[string]any{}}
	}
	return e.cfg
}

// SetConfig applies cfg to the extractor. Returns an error for unknown options.
func (e *StackoverflowExtractor) SetConfig(c *config.Extractor) error {
	for k := range c.Options {
		return fmt.Errorf("unknown option %q", k)
	}
	e.cfg = c
	return nil
}

func (e *StackoverflowExtractor) Match(d *document.Document) bool {
	return strings.HasPrefix(d.URL, matchURLPrefix) && len(d.URL) > len(matchURLPrefix)
}

func (e *StackoverflowExtractor) Extract(d *document.Document) (types.ExtractorState, error) {
	return types.ExtractorContinue, nil
}

func (e *StackoverflowExtractor) Preview(d *document.Document) (types.PreviewResponse, types.ExtractorState, error) {
	// TODO include more details about the question/answers
	// TODO rewrite URLs and references to absolute
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(d.HTML))
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}

	// remove the "copy" panel
	doc.Find("pre div").Remove()

	question, err := doc.Find(".js-post-body").Html()
	if err != nil {
		return types.PreviewResponse{}, types.ExtractorContinue, err
	}

	answers := make([]string, 0)
	for i, a := range doc.Find(".answercell .s-prose").EachIter() {
		h, err := a.Html()
		if err != nil {
			continue
		}
		answers = append(answers, fmt.Sprintf("<h2>Answer #%d</h2>%s", i+1, h))
	}
	res := fmt.Sprintf(
		"<h2>Question</h2>%s%s",
		question,
		strings.Join(answers, "<hr />"),
	)
	return types.PreviewResponse{Content: sanitizer.SanitizeHTML(res)}, types.ExtractorStop, nil
}
