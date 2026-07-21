package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/server/document"
	"github.com/asciimoo/hister/server/indexer"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

const (
	linkwardenTokenEnv            = "HISTER_IMPORT_LINKWARDEN_TOKEN"
	linkwardenSourceMetadataValue = "linkwarden"
	linkwardenSourceQuery         = "metadata.source:" + linkwardenSourceMetadataValue
	linkwardenRequestTimeout      = 30 * time.Second
	maxLinkwardenResponseSize     = 64 << 20
	maxLinkwardenErrorBodySize    = 64 << 10
)

var errLinkwardenMissingURL = errors.New("linkwarden record has no URL")

type linkwardenTag struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type linkwardenCollection struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type linkwardenLink struct {
	ID          int64                 `json:"id"`
	Name        string                `json:"name"`
	Type        string                `json:"type"`
	Description string                `json:"description"`
	URL         string                `json:"url"`
	TextContent string                `json:"textContent"`
	ImportDate  string                `json:"importDate"`
	CreatedAt   string                `json:"createdAt"`
	UpdatedAt   string                `json:"updatedAt"`
	Tags        []linkwardenTag       `json:"tags"`
	Collection  *linkwardenCollection `json:"collection"`
}

type linkwardenSearchData struct {
	NextCursor *int             `json:"nextCursor"`
	Links      []linkwardenLink `json:"links"`
}

type linkwardenSearchResponse struct {
	Data *linkwardenSearchData `json:"data"`
}

type linkwardenClient struct {
	searchURL    string
	token        string
	updatedAfter int64
	httpClient   *http.Client
}

type linkwardenImportOptions struct {
	BatchSize    int
	SkipExisting bool
	StartDate    int64
	EndDate      int64
}

type linkwardenImportStats struct {
	Imported int
	Skipped  int
	Errors   int
}

var importLinkwardenCmd = &cobra.Command{
	Use:   "linkwarden INSTANCE_URL",
	Short: "Import bookmarks from Linkwarden",
	Long: `Import bookmarks and their searchable content from a Linkwarden instance.

Set the Linkwarden API token with HISTER_IMPORT_LINKWARDEN_TOKEN or --api-token.
The global --token flag remains the access token for the destination Hister server.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		token := linkwardenAPIToken(cmd)
		if token == "" {
			exit(1, "Linkwarden API token is required; set "+linkwardenTokenEnv+" or use --api-token")
		}

		batchSize, _ := cmd.Flags().GetInt("batch-size")
		if batchSize < 1 || batchSize > maxImportBatchSize {
			exit(1, fmt.Sprintf("--batch-size must be between 1 and %d", maxImportBatchSize))
		}
		dateRange, err := parseDateRangeFlags(cmd)
		if err != nil {
			exit(1, err.Error())
		}
		source, err := newLinkwardenClient(args[0], token, nil)
		if err != nil {
			exit(1, err.Error())
		}

		global, _ := cmd.Flags().GetBool("global")
		clientOpts := append([]client.Option{client.WithTimeout(0)}, targetUserIDClientOptions(cmd, global)...)
		target := newClient(clientOpts...)
		updatedAfter, err := latestLinkwardenUpdated(target)
		if err != nil {
			exit(1, "Failed to find the latest Linkwarden import: "+err.Error())
		}
		source.updatedAfter = updatedAfter
		skipExisting, _ := cmd.Flags().GetBool("skip-existing")
		languageDetector := document.LanguageDetector(document.NewNullLanguageDetector())
		if cfg.Indexer.DetectLanguages {
			languageDetector = document.NewLanguageDetector()
		}

		stats, err := importLinkwarden(cmd.Context(), source, target, languageDetector, linkwardenImportOptions{
			BatchSize:    batchSize,
			SkipExisting: skipExisting,
			StartDate:    dateRange.From,
			EndDate:      dateRange.To,
		})
		if err != nil {
			exit(1, "Linkwarden import failed: "+err.Error())
		}
		printImportSummary(stats.Imported, stats.Skipped, stats.Errors)
	},
}

func linkwardenAPIToken(cmd *cobra.Command) string {
	if cmd.Flags().Changed("api-token") {
		token, _ := cmd.Flags().GetString("api-token")
		return strings.TrimSpace(token)
	}
	return strings.TrimSpace(os.Getenv(linkwardenTokenEnv))
}

func newLinkwardenClient(instanceURL, token string, httpClient *http.Client) (*linkwardenClient, error) {
	instanceURL = strings.TrimSpace(instanceURL)
	parsed, err := url.Parse(instanceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid Linkwarden instance URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("invalid Linkwarden instance URL: an http or https URL with a host is required")
	}
	if parsed.User != nil {
		return nil, errors.New("invalid Linkwarden instance URL: embedded credentials are not supported")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("invalid Linkwarden instance URL: query parameters and fragments are not supported")
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: linkwardenRequestTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return errors.New("stopped after 10 redirects")
				}
				origin := via[0].URL
				if !strings.EqualFold(req.URL.Scheme, origin.Scheme) || !strings.EqualFold(req.URL.Host, origin.Host) {
					return errors.New("refusing to send the Linkwarden token to a different origin")
				}
				return nil
			},
		}
	}

	return &linkwardenClient{
		searchURL:  strings.TrimRight(parsed.String(), "/") + "/api/v1/search",
		token:      token,
		httpClient: httpClient,
	}, nil
}

func (c *linkwardenClient) search(ctx context.Context, cursor *int) (*linkwardenSearchData, error) {
	requestURL, err := url.Parse(c.searchURL)
	if err != nil {
		return nil, fmt.Errorf("build Linkwarden search URL: %w", err)
	}
	query := requestURL.Query()
	if c.updatedAfter != 0 {
		query.Set("searchQueryString", "after:"+time.Unix(c.updatedAfter, 0).UTC().Format("2006-01-02"))
	}
	if cursor != nil {
		query.Set("cursor", strconv.Itoa(*cursor))
	}
	requestURL.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create Linkwarden request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request Linkwarden links: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Debug().Err(closeErr).Msg("Failed to close Linkwarden response body")
		}
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxLinkwardenErrorBodySize))
		detail := strings.TrimSpace(string(body))
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("linkwarden authentication failed with status %d; check %s or --api-token", resp.StatusCode, linkwardenTokenEnv)
		default:
			if detail != "" {
				return nil, fmt.Errorf("linkwarden returned status %d: %s", resp.StatusCode, detail)
			}
			return nil, fmt.Errorf("linkwarden returned status %d", resp.StatusCode)
		}
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLinkwardenResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("read Linkwarden response: %w", err)
	}
	if len(body) > maxLinkwardenResponseSize {
		return nil, fmt.Errorf("linkwarden response exceeds the %d MiB limit", maxLinkwardenResponseSize>>20)
	}
	var response linkwardenSearchResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("decode Linkwarden response: %w", err)
	}
	if response.Data == nil {
		return nil, errors.New("linkwarden response is missing data")
	}
	return response.Data, nil
}

func latestLinkwardenUpdated(target *client.Client) (int64, error) {
	result, err := target.Search(&indexer.Query{
		Text:  linkwardenSourceQuery,
		Limit: 1,
		Sort:  "date",
	})
	if err != nil {
		return 0, err
	}
	if len(result.Documents) == 0 {
		return 0, nil
	}
	return result.Documents[0].Updated, nil
}

func importLinkwarden(
	ctx context.Context,
	source *linkwardenClient,
	target *client.Client,
	languageDetector document.LanguageDetector,
	options linkwardenImportOptions,
) (linkwardenImportStats, error) {
	var stats linkwardenImportStats
	if options.BatchSize < 1 || options.BatchSize > maxImportBatchSize {
		return stats, fmt.Errorf("batch size must be between 1 and %d", maxImportBatchSize)
	}
	var cursor *int
	seenCursors := make(map[int]struct{})
	docs := make([]*document.Document, 0, options.BatchSize)
	flush := func() {
		if len(docs) == 0 {
			return
		}
		imported, errCount := addDocumentBatch(target, docs)
		stats.Imported += imported
		stats.Errors += errCount
		docs = docs[:0]
	}

	for {
		page, err := source.search(ctx, cursor)
		if err != nil {
			flush()
			return stats, err
		}

		for _, link := range page.Links {
			d, err := linkwardenDocument(link, languageDetector)
			if errors.Is(err, errLinkwardenMissingURL) {
				log.Debug().Int64("linkwarden_id", link.ID).Msg("Skipping Linkwarden record without a URL")
				stats.Skipped++
				continue
			}
			if err != nil {
				log.Warn().Err(err).Int64("linkwarden_id", link.ID).Msg("Failed to convert Linkwarden record, skipping")
				stats.Errors++
				continue
			}
			if (options.StartDate != 0 && d.Added < options.StartDate) || (options.EndDate != 0 && d.Added > options.EndDate) {
				log.Debug().Str("url", d.URL).Int64("added", d.Added).Msg("Skipping Linkwarden record outside of date range")
				stats.Skipped++
				continue
			}
			if options.SkipExisting {
				exists, err := target.DocumentExists(d.URL)
				if err != nil {
					log.Warn().Err(err).Str("url", d.URL).Msg("Failed to check if document exists, skipping")
					stats.Errors++
					continue
				}
				if exists {
					log.Debug().Str("url", d.URL).Msg("Document already exists, skipping")
					stats.Skipped++
					continue
				}
			}

			docs = append(docs, d)
			if len(docs) == options.BatchSize {
				flush()
			}
		}

		if page.NextCursor == nil {
			flush()
			return stats, nil
		}
		if _, seen := seenCursors[*page.NextCursor]; seen {
			flush()
			return stats, fmt.Errorf("linkwarden returned the repeated pagination cursor %d", *page.NextCursor)
		}
		seenCursors[*page.NextCursor] = struct{}{}
		cursor = page.NextCursor
	}
}

func linkwardenDocument(link linkwardenLink, languageDetector document.LanguageDetector) (*document.Document, error) {
	linkURL := strings.TrimSpace(link.URL)
	if linkURL == "" {
		return nil, errLinkwardenMissingURL
	}

	metadata := map[string]any{
		"source":          linkwardenSourceMetadataValue,
		"linkwarden_id":   link.ID,
		"linkwarden_type": link.Type,
	}
	if description := strings.TrimSpace(link.Description); description != "" {
		metadata["description"] = description
	}
	if link.Collection != nil {
		metadata["linkwarden_collection"] = link.Collection.Name
		metadata["linkwarden_collection_id"] = link.Collection.ID
	}
	if len(link.Tags) > 0 {
		tags := make([]string, 0, len(link.Tags))
		for _, tag := range link.Tags {
			if name := strings.TrimSpace(tag.Name); name != "" {
				tags = append(tags, name)
			}
		}
		if len(tags) > 0 {
			metadata["linkwarden_tags"] = tags
		}
	}

	added := parseLinkwardenTime(link.ImportDate)
	if added == 0 {
		added = parseLinkwardenTime(link.CreatedAt)
	}
	d := &document.Document{
		URL:      linkURL,
		Title:    strings.TrimSpace(link.Name),
		Text:     combineLinkwardenText(link.Description, link.TextContent),
		Added:    added,
		Updated:  parseLinkwardenTime(link.UpdatedAt),
		Metadata: metadata,
	}
	sourceUpdated := d.Updated
	if err := d.Process(languageDetector, nil); err != nil {
		return nil, err
	}
	if d.Title == "" {
		d.Title = d.URL
	}
	if sourceUpdated != 0 {
		d.Updated = sourceUpdated
	}
	return d, nil
}

func combineLinkwardenText(description, textContent string) string {
	description = strings.TrimSpace(description)
	textContent = strings.TrimSpace(textContent)
	if description == "" || description == textContent {
		return textContent
	}
	if textContent == "" {
		return description
	}
	return description + "\n\n" + textContent
}

func parseLinkwardenTime(value string) int64 {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Unix()
		}
	}
	return 0
}
