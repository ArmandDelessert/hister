package cmd

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/asciimoo/hister/client"
	"github.com/asciimoo/hister/server/crawler"
	"github.com/asciimoo/hister/server/model"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var browserImportCmd = &cobra.Command{
	Use:   "import-browser [BROWSER_TYPE] [DB_PATH]",
	Short: "Import Chrome, Firefox or auto-detect browsing history",
	Long: `
Import browsing history from a supported browser.

Usage:
  import-browser                        - auto-detect all installed browsers
  import-browser BROWSER_TYPE           - auto-detect database path
  import-browser DB_PATH                - auto-detect browser type
  import-browser BROWSER_TYPE DB_PATH   - import a browser type with a specific database path

Supported for browser types for auto-detecting: firefox, chrome, chromium, brave, edge, vivaldi, opera, zen, waterfox, Ladybird

The Firefox URL database is usually located at ~/.mozilla/firefox/*.default/places.sqlite
The Chrome/Chromium URL database is usually located at ~/.config/chromium/Default/History
`,
	Args: cobra.RangeArgs(0, 2),
	PreRun: func(_ *cobra.Command, _ []string) {
		initDB()
		initExtractor()
	},
	Run: importHistory,
}

type browserDBCandidates struct {
	name             string
	table_name       string
	paths_candidates []string
}

type browserDB struct {
	name       string
	table_name string
	paths      []string
}

type importHistoryMultipleChoicePrompt struct {
	choice string
	urls   int
	db     *sql.DB
	q      string
	c      *client.Client
}

type DBToImport struct {
	name         string
	table        string
	databaseFile string
	browserType  string
	db           *sql.DB
	q            string
	c            *client.Client
	count        int
}

type browserImportJob struct {
	id       string
	startURL string
	created  bool
	enqueued int
}

const browserImportJobPrefix = "browser-import-"

func importHistory(cmd *cobra.Command, args []string) {
	// TODO: get skip rules from server
	cfg.Crawler.UserAgent = UserAgent
	applyCrawlerBackendFlags(cmd)

	switch len(args) {
	case 0:
		// Auto-detect all installed browsers.
		dbs := getDBPaths()
		if len(dbs) == 0 {
			log.Fatal().Msg("no browser databases found")
		}
		var databases []DBToImport
		for _, db := range dbs {
			for _, path := range db.paths {
				databases = append(databases, DBToImport{
					table:        db.table_name,
					databaseFile: path,
				})
			}
		}
		importDB(databases, cmd)

	case 1, 2:
		if len(args) == 1 {
			// check if args[0] is a file or not and call the correct function
			if _, err := os.Stat(args[0]); os.IsNotExist(err) {
				importBrowser(strings.ToLower(args[0]), cmd)
			} else {
				importHistoryFile(args[0], cmd)
			}
		} else {
			browser := args[0]
			table_name := browserTableName(browser)
			if table_name == "" {
				log.Warn().Msg(fmt.Sprintf("Unknown browser, couldn't auto detect table name using %s as table name", browser))
				table_name = browser
			}
			importDB([]DBToImport{
				{
					table:        table_name,
					databaseFile: args[1],
				},
			},
				cmd)
		}

	default:
		log.Fatal().Msg(cmd.Long)
	}

	// TODO optional date filter
	//vf := "last_visit_time"
	//if browser == "firefox" {
	//	vf = "last_visit_date"
	//}
	//q += fmt.Sprintf(" AND %s >= datetime('now', 'localtime', '-1 month')", vf)
}

func importBrowser(browser string, cmd *cobra.Command) {
	var found bool

	for _, db := range getDBPaths() {
		if strings.HasPrefix(strings.ToLower(db.name), browser) {
			found = true
			for _, path := range db.paths {
				importDB([]DBToImport{
					{
						table:        db.table_name,
						databaseFile: path,
					},
				},
					cmd)
			}
		}
	}
	if !found {
		log.Fatal().Str("browser", browser).Msg("no database found for browser")
	}
}

func importHistoryFile(file_path string, cmd *cobra.Command) {
	var table string

	if strings.HasSuffix(file_path, "places.sqlite") {
		table = "moz_places"
	} else if strings.HasSuffix(file_path, "History") {
		table = "urls"
	} else if strings.HasSuffix(file_path, "History.db") {
		table = "History"
	} else {
		log.Fatal().Str("file", file_path).Msg("Couldn't auto detect table")
	}

	importDB([]DBToImport{
		{
			table:        table,
			databaseFile: file_path,
		},
	},
		cmd)
}

func importDB(databases []DBToImport, cmd *cobra.Command) {
	var dbsToImport []importHistoryMultipleChoicePrompt
	for _, database := range databases {
		dbFile := database.databaseFile
		table := database.table

		db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?immutable=1&mode=ro", dbFile))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to open database")
		}
		defer func() {
			if err := db.Close(); err != nil {
				log.Warn().Err(err).Msg("failed to close database")
			}
		}()

		// Fetch skip rules from the server.
		c := newClient()
		resp, err := c.FetchRules()
		if err != nil {
			log.Error().Err(err).Msg("Unable to obtain skip rules from server; using local ones instead")
		} else {
			// TODO: let the user know that their local rules are being overwritten?
			cfg.Rules.Skip.ReStrs = resp.Skip
			if err := cfg.Rules.Skip.Compile(); err != nil {
				log.Error().Err(err).Msg("Unable to compile skip rules from server")
				return
			}
		}

		q := fmt.Sprintf("SELECT DISTINCT count(url) FROM %s WHERE url LIKE 'http://%%' OR url LIKE 'https://%%'", table)
		if i, err := cmd.Flags().GetInt("min-visit"); err == nil && i > 1 {
			q += fmt.Sprintf(" AND visit_count >= %d", i)
		}
		// TODO: apply skip rules to get a more precise count?
		row := db.QueryRow(q)
		var count int
		if err := row.Scan(&count); err != nil {
			log.Debug().Str("query", q).Msg("count query")
			log.Error().Err(err).Msg("Failed to execute counting query")
			return
		}

		if count < 1 {
			exit(1, "No URLs found to import")
		}
		dbsToImport = append(dbsToImport, importHistoryMultipleChoicePrompt{dbFile, count, db, q, c})
		// if !yesNoPrompt(fmt.Sprintf("%d URLs found. Start import form "+dbFile, count), true) {
		// 	return
		// }
	}

	chosen := multipleChoiceImport(dbsToImport)

	defaultJobID := browserImportJobPrefix + time.Now().Format("2006-01-02")
	jobID, resumeExisting, err := chooseBrowserImportJobID(defaultJobID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to select browser import crawl job")
		return
	}
	job := &browserImportJob{id: jobID}
	if resumeExisting {
		if err := ensureBrowserImportJob(job, ""); err != nil {
			log.Error().Err(err).Msg("Failed to resume browser import crawl job")
			return
		}
	}

	for _, database := range chosen {
		q := database.q
		count := database.count
		db := database.db

		q = strings.Replace(q, "count(url)", "url", 1)
		q += " ORDER BY visit_count DESC"

		rows, err := db.Query(q)
		if err != nil {
			log.Error().Err(err).Msg("Failed to execute database query")
			return
		}
		defer func() {
			if err := rows.Close(); err != nil {
				log.Warn().Err(err).Msg("failed to close database rows")
			}
		}()
		i := 0
		skippedByRules := 0
		batch := make([]string, 0, 500)
		for rows.Next() {
			i += 1
			var u string
			err = rows.Scan(&u)
			if err != nil {
				log.Error().Err(err).Msg("Failed to scan database row")
				return
			}
			// skip URLs only in single user environments
			if !cfg.App.UserHandling && cfg.Rules.IsSkip(u) {
				log.Debug().Str("URL", u).Msg("skip importing URL by rule")
				skippedByRules += 1
				continue
			}
			if err := ensureBrowserImportJob(job, u); err != nil {
				log.Error().Err(err).Msg("Failed to create browser import crawl job")
				return
			}
			batch = append(batch, u)
			if len(batch) >= cap(batch) {
				if err := model.BulkInsertCrawlURLs(job.id, batch, 0); err != nil {
					log.Error().Err(err).Msg("Failed to add browser URLs to crawl job")
					return
				}
				job.enqueued += len(batch)
				batch = batch[:0]
			}
		}
		if err := rows.Err(); err != nil {
			log.Error().Err(err).Msg("Failed to read browser URLs")
			return
		}
		if len(batch) > 0 {
			if err := model.BulkInsertCrawlURLs(job.id, batch, 0); err != nil {
				log.Error().Err(err).Msg("Failed to add browser URLs to crawl job")
				return
			}
			job.enqueued += len(batch)
		}
		if skippedByRules != 0 {
			log.Info().Msgf("Skipped %d URLs by rules", skippedByRules)
		}
		log.Info().Str("job_id", job.id).Int("seen", i).Int("total", count).Msg("Browser URLs added to crawl job")
	}

	if !job.created {
		exit(1, "No URLs found to import")
	}
	storedJob, err := model.GetCrawlJob(job.id)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load browser import crawl job")
		return
	}
	if storedJob == nil {
		log.Error().Str("job_id", job.id).Msg("Browser import crawl job not found")
		return
	}
	hasURLs, err := crawlJobHasURLsToCrawl(storedJob)
	if err != nil {
		log.Error().Err(err).Msg("Failed to load browser import crawl job queue")
		return
	}
	if !hasURLs {
		fmt.Println("No URLs to crawl for job:", job.id)
		return
	}

	fmt.Println(cliBoldStyle.Render("IMPORTING"))
	fmt.Println("Starting crawl job:", job.id)

	cfg.Crawler.UserAgent = UserAgent
	cr, err := crawler.NewPersistent(&cfg.Crawler, job.id, nil, crawlerSkipOptions(false)...)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize persistent crawler")
	}
	defer func() {
		if err := cr.Close(); err != nil {
			log.Warn().Err(err).Msg("crawler close error")
		}
	}()

	validatorRules := &crawler.ValidatorRules{NoDepth: true}
	validator, err := crawler.NewValidator(validatorRules)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid browser import crawler rules")
	}
	done, err := model.CountCrawlURLsByStatus(job.id, model.CrawlURLDone)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to count done browser import URLs")
	}
	failed, err := model.CountCrawlURLsByStatus(job.id, model.CrawlURLFailed)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to count failed browser import URLs")
	}
	validator.SetVisited(int(done + failed))

	if err := crawlAndIndex(job.id, job.startURL, cr, validator, ""); err != nil {
		log.Fatal().Err(err).Msg("Browser import crawl failed")
	}
}

func ensureBrowserImportJob(job *browserImportJob, startURL string) error {
	if job.created {
		return nil
	}
	rules := &crawler.ValidatorRules{NoDepth: true}
	rulesJSON, err := crawler.MarshalValidatorRules(rules)
	if err != nil {
		return fmt.Errorf("serialize browser import crawler rules: %w", err)
	}
	existing, err := model.GetCrawlJob(job.id)
	if err != nil {
		return fmt.Errorf("load crawl job: %w", err)
	}
	if existing == nil {
		if err := model.CreateCrawlJob(job.id, startURL, rulesJSON, ""); err != nil {
			return fmt.Errorf("create crawl job: %w", err)
		}
		job.startURL = startURL
		job.created = true
		return nil
	}
	existingRules, err := crawler.UnmarshalValidatorRules(existing.ValidatorRules)
	if err != nil {
		return fmt.Errorf("restore crawl job rules: %w", err)
	}
	if !existingRules.NoDepth {
		return fmt.Errorf("crawl job %q already exists and is not a browser import job", job.id)
	}
	if err := model.UpdateCrawlJobStatus(job.id, model.CrawlJobRunning); err != nil {
		return fmt.Errorf("update crawl job status: %w", err)
	}
	job.startURL = existing.StartURL
	job.created = true
	return nil
}

func chooseBrowserImportJobID(defaultID string) (string, bool, error) {
	jobs, err := model.ListCrawlJobs()
	if err != nil {
		return "", false, fmt.Errorf("list crawl jobs: %w", err)
	}
	browserJobs := browserImportJobs(jobs)
	if len(browserJobs) == 0 {
		id, err := nextBrowserImportJobID(defaultID)
		return id, false, err
	}
	if selected := promptBrowserImportJob(browserJobs, defaultID); selected != "" {
		return selected, true, nil
	}
	id, err := nextBrowserImportJobID(defaultID)
	return id, false, err
}

func browserImportJobs(jobs []*model.CrawlJob) []*model.CrawlJob {
	var browserJobs []*model.CrawlJob
	for _, job := range jobs {
		if !strings.HasPrefix(job.ID, browserImportJobPrefix) {
			continue
		}
		rules, err := crawler.UnmarshalValidatorRules(job.ValidatorRules)
		if err != nil {
			log.Warn().Err(err).Str("job_id", job.ID).Msg("failed to restore crawl job rules")
			continue
		}
		if !rules.NoDepth {
			continue
		}
		browserJobs = append(browserJobs, job)
	}
	return browserJobs
}

func promptBrowserImportJob(jobs []*model.CrawlJob, defaultID string) string {
	r := bufio.NewReader(os.Stdin)
	if len(jobs) == 1 {
		job := jobs[0]
		fmt.Println("Existing browser import job found:")
		printBrowserImportJob(1, job)
		if yesNoPrompt(fmt.Sprintf("Continue this job instead of creating %s?", defaultID), true) {
			return job.ID
		}
		return ""
	}

	fmt.Println("Existing browser import jobs found:")
	for i, job := range jobs {
		printBrowserImportJob(i+1, job)
	}
	fmt.Printf("Choose job number to continue, or press enter to create %s: ", defaultID)
	answer, _ := r.ReadString('\n')
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return ""
	}
	selected, err := strconv.Atoi(answer)
	if err != nil || selected < 1 || selected > len(jobs) {
		fmt.Println("Invalid selection, creating a new browser import job.")
		return ""
	}
	return jobs[selected-1].ID
}

func printBrowserImportJob(idx int, job *model.CrawlJob) {
	stats, err := model.GetCrawlJobStats(job.ID)
	if err != nil {
		log.Warn().Err(err).Str("job_id", job.ID).Msg("failed to get job stats")
	}
	fmt.Printf("%d  %s  %s\n", idx, job.ID, crawlJobStatusLabel(job.Status))
	fmt.Printf("   pending: %d  done: %d  failed: %d  skipped: %d  created: %s\n",
		stats.Pending, stats.Done, stats.Failed, stats.Skipped,
		job.CreatedAt.Format("2006-01-02 15:04:05"),
	)
}

func nextBrowserImportJobID(baseID string) (string, error) {
	job, err := model.GetCrawlJob(baseID)
	if err != nil {
		return "", fmt.Errorf("load crawl job: %w", err)
	}
	if job == nil {
		return baseID, nil
	}
	for i := 2; ; i++ {
		id := fmt.Sprintf("%s-%d", baseID, i)
		job, err := model.GetCrawlJob(id)
		if err != nil {
			return "", fmt.Errorf("load crawl job: %w", err)
		}
		if job == nil {
			return id, nil
		}
	}
}

func getDBPaths() []browserDB {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var candidates []browserDBCandidates

	chromium_table := "urls"
	firefox_table := "moz_places"
	ladybird_table := "History"

	switch runtime.GOOS {
	default:
		log.Fatal().Msgf("Failed to detect os")
	case "darwin":
		candidates = []browserDBCandidates{
			// firefox
			{
				"Firefox",
				firefox_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles", "*.default*", "places.sqlite"),
					filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles", "*.default-release*", "places.sqlite"),
				},
			},
			{
				"Firefox Developer Edition",
				firefox_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Firefox", "Profiles", "*.dev-edition-default*", "places.sqlite"),
				},
			},
			{
				"Zen",
				firefox_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "zen", "Profiles", "*Default*", "places.sqlite"),
				},
			},
			{
				"Waterfox",
				firefox_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Waterfox", "Profiles", "*.default*", "places.sqlite"),
				},
			},
			{
				"Ladybird",
				ladybird_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Ladybird", "History.db"),
				},
			},
			{
				"Chrome",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "Default", "History"),
					filepath.Join(home, "Library", "Application Support", "Google", "Chrome Beta", "Default", "History"),
					filepath.Join(home, "Library", "Application Support", "Google", "Chrome Canary", "Default", "History"),
				},
			},
			{
				"Chromium",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Chromium", "Default", "History"),
				},
			},
			{
				"Brave",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser", "Default", "History"),
					filepath.Join(home, "Library", "Application Support", "BraveSoftware", "Brave-Browser-Beta", "Default", "History"),
				},
			},
			{
				"Edge",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Microsoft Edge", "Default", "History"),
					filepath.Join(home, "Library", "Application Support", "Microsoft Edge Beta", "Default", "History"),
				},
			},
			{
				"Vivaldi",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "Vivaldi", "Default", "History"),
				},
			},
			{
				"Opera",
				chromium_table,
				[]string{
					filepath.Join(home, "Library", "Application Support", "com.operasoftware.Opera", "Default", "History"),
				},
			},
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		appData := os.Getenv("APPDATA")
		if localAppData != "" {
			candidates = []browserDBCandidates{
				{
					"firefox",
					firefox_table,
					[]string{
						filepath.Join(appData, "Mozilla", "Firefox", "Profiles", "*.default*", "places.sqlite"),
						filepath.Join(appData, "Mozilla", "Firefox", "Profiles", "*.default-release*", "places.sqlite"),
					},
				},
				{
					"Zen",
					firefox_table,
					[]string{
						filepath.Join(appData, "zen", "Profiles", "*.Default*", "places.sqlite"),
					},
				},
				{
					"Waterfox",
					firefox_table,
					[]string{
						filepath.Join(appData, "Waterfox", "Profiles", "*.default*", "places.sqlite"),
					},
				},
				{
					"Chrome",
					chromium_table,
					[]string{
						filepath.Join(localAppData, "Google", "Chrome", "User Data", "Default", "History"),
						filepath.Join(localAppData, "Google", "Chrome Beta", "User Data", "Default", "History"),
					},
				},
				{
					"Chromium",
					chromium_table,
					[]string{
						filepath.Join(localAppData, "Chromium", "User Data", "Default", "History"),
					},
				},
				{
					"Brave",
					chromium_table,
					[]string{
						filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data", "Default", "History"),
					},
				},
				{
					"Edge",
					chromium_table,
					[]string{
						filepath.Join(localAppData, "Microsoft", "Edge", "User Data", "Default", "History"),
					},
				},
				{
					"Vivaldi",
					chromium_table,
					[]string{
						filepath.Join(localAppData, "Vivaldi", "User Data", "Default", "History"),
					},
				},
				{
					"Opera",
					chromium_table,
					[]string{
						filepath.Join(appData, "Opera Software", "Opera Stable", "History"),
					},
				},
			}
		}
	case "linux":
		candidates = []browserDBCandidates{
			{
				"firefox",
				firefox_table,
				[]string{
					filepath.Join(home, "snap", "firefox", "common", ".mozilla", "firefox", "*.default*", "places.sqlite"),
					filepath.Join(home, ".mozilla", "firefox", "*.default*", "places.sqlite"),
				},
			},
			{
				"Firefox Developer Edition",
				firefox_table,
				[]string{
					filepath.Join(home, ".mozilla", "firefox", "*.dev-edition-default*", "places.sqlite"),
				},
			},
			{
				"Zen",
				firefox_table,
				[]string{
					filepath.Join(home, ".zen", "*.Default*", "places.sqlite"),
					filepath.Join(home, ".config", "zen", "*.Default*", "places.sqlite"),
				},
			},
			{
				"Waterfox",
				firefox_table,
				[]string{
					filepath.Join(home, ".waterfox", "Profiles", "*.default*", "places.sqlite"),
				},
			},
			{
				"Ladybird",
				ladybird_table,
				[]string{
					filepath.Join(home, ".local", "share", "Ladybird", "History.db"),
				},
			},
			{
				"Chrome",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "google-chrome", "Default", "History"),
					filepath.Join(home, ".config", "google-chrome-beta", "Default", "History"),
				},
			},
			{
				"Chromium",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "chromium", "Default", "History"),
					filepath.Join(home, "snap", "chromium", "common", "chromium", "Default", "History"),
				},
			},
			{
				"Brave",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser", "Default", "History"),
				},
			},
			{
				"Edge",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "microsoft-edge", "Default", "History"),
					filepath.Join(home, ".config", "microsoft-edge-beta", "Default", "History"),
				},
			},
			{
				"Vivaldi",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "vivaldi", "Default", "History"),
				},
			},
			{
				"Opera",
				chromium_table,
				[]string{
					filepath.Join(home, ".config", "opera", "Default", "History"),
				},
			},
		}
	}

	var dbFiles []browserDB
	var paths []string

	for _, candidate := range candidates {
		for _, globs := range candidate.paths_candidates {
			matches, _ := filepath.Glob(globs)
			for _, p := range matches {
				if _, err := os.Stat(p); err == nil {
					paths = append(paths, p)
				}
			}
		}

		if len(paths) != 0 {
			dbFiles = append(dbFiles, browserDB{candidate.name, candidate.table_name, paths})
		}
		paths = []string{}
	}
	return dbFiles
}

func browserTableName(browser string) string {
	switch strings.ToLower(browser) {
	case "firefox", "zen", "waterfox":
		return "moz_places"
	case "chrome", "chromium", "brave", "edge", "vivaldi", "opera":
		return "urls"
	case "ladybird":
		return "History"
	}
	return ""
}

func multipleChoiceImport(choices []importHistoryMultipleChoicePrompt) []DBToImport {
	r := bufio.NewReader(os.Stdin)
	var s string
	var returnDBs []DBToImport
	println("----Available Histories----")
	for i, choiceData := range choices {
		prefix := getBrowserType(choiceData.choice)
		choice := fmt.Sprint(strconv.Itoa(i), "  |  ", prefix, "  ", choiceData.choice, "  urls: ", choiceData.urls)
		println(choice)
		returnDBs = append(returnDBs, DBToImport{
			name:        prefix,
			browserType: prefix,
			count:       choiceData.urls,
			db:          choiceData.db,
			q:           choiceData.q,
			c:           choiceData.c,
		})
	}
	println("==> Histories to exclude: (eg: \"1 2 3\", browser name or leave empty to to import all)")
	print("==> ")

	s, _ = r.ReadString('\n')

	blacklists := strings.Split(strings.Trim(s, "\n"), " ")

	// Handle blacklisted imports
	var selected []DBToImport
	var unselected bool
	for i, data := range returnDBs {
		for _, blacklist := range blacklists {
			if strconv.Itoa(i) == blacklist || data.name == blacklist {
				unselected = true
				break
			}
		}
		if !unselected {
			selected = append(selected, data)
		}
		unselected = false
	}
	return selected
}

func getBrowserType(path string) string {
	path = strings.ToLower(path)
	if strings.Contains(path, "firefox") {
		return "firefox"
	} else if strings.Contains(path, "zen") {
		return "zen"
	} else if strings.Contains(path, "waterfox") {
		return "waterfox"
	} else if strings.Contains(path, "chrome") {
		return "chrome"
	} else if strings.Contains(path, "chromium") {
		return "chromium"
	} else if strings.Contains(path, "brave") {
		return "brave"
	} else if strings.Contains(path, "edge") {
		return "edge"
	} else if strings.Contains(path, "vivaldi") {
		return "vivaldi"
	} else if strings.Contains(path, "opera") {
		return "opera"
	} else if strings.Contains(path, "ladybird") {
		return "ladybird"
	} else {
		return "unknown"
	}
}
