package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadCommentedConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name, content, address string
	}{
		{"comments only", "# app:\n#   title: Example title\n# server:\n#   address: 127.0.0.1:5544\n", DefaultServerAddress},
		{"selected setting", "# app:\n#   title: Example title\nserver:\n  address: 127.0.0.1:5544\n", "127.0.0.1:5544"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, inspection := range []bool{false, true} {
				name := "startup"
				if inspection {
					name = "inspection"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					filename := filepath.Join(dir, "config.yml")
					t.Setenv("HISTER_DATA_DIR", filepath.Join(dir, "data"))
					if err := os.WriteFile(filename, []byte(tc.content), 0o600); err != nil {
						t.Fatal(err)
					}
					var c *Config
					var err error
					if inspection {
						var selected string
						c, selected, err = LoadForInspection(filename, true)
						if selected != filename {
							t.Fatalf("selected file = %q, want %q", selected, filename)
						}
					} else {
						c, err = Load(filename)
					}
					if err != nil {
						t.Fatal(err)
					}
					if err := c.Validate(); err != nil {
						t.Fatal(err)
					}
					if c.App.Title != CreateDefaultConfig().App.Title || c.Server.Address != tc.address {
						t.Fatalf("title = %q, address = %q; commented values must not override defaults", c.App.Title, c.Server.Address)
					}
					content, err := os.ReadFile(filename)
					if err != nil || string(content) != tc.content {
						t.Fatalf("loading changed main configuration: %q, err=%v", content, err)
					}
					if inspection {
						entries, err := os.ReadDir(dir)
						if err != nil || len(entries) != 1 {
							t.Fatalf("inspection created runtime files: %v, err=%v", entries, err)
						}
					}
				})
			}
		})
	}
}

func TestTUIConfigurationDoesNotSelectMainFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	c := CreateDefaultConfig()
	c.TUI = DefaultTUIConfig
	c.TUI.DarkTheme = "saved-theme"
	if err := c.SaveTUIConfig(); err != nil {
		t.Fatal(err)
	}
	if c.Filename() != "*Default Config*" {
		t.Fatalf("saving TUI preferences selected a nonexistent main file: %q", c.Filename())
	}
	c = CreateDefaultConfig()
	c.LoadTUIConfig()
	if c.Filename() != "*Default Config*" || c.TUI.DarkTheme != "saved-theme" {
		t.Fatalf("TUI loading lost preferences or selected a main file: %q, %+v", c.Filename(), c.TUI)
	}
}

func TestConfigurationSourcesPreserveExplicitValues(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	const raw = `app:
  title: Hister
  subtitle: ''
  show_suggestions: false
crawler:
  delay: 0
  headers: {}
  cookies: []
indexer:
  languages: []
server: {}
extractors:
  markdown: {}
  ytdlp:
    options:
      timeout: 0
`
	if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := LoadForInspection(filename, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"app.title", "app.subtitle", "app.show_suggestions", "crawler.delay", "crawler.headers", "crawler.cookies", "indexer.languages", "extractors.ytdlp.options.timeout"} {
		source := c.Source(path)
		if source.Kind != "file" || source.Name != filename || !source.Locked || source.Reason == "" {
			t.Errorf("explicit %s source = %+v, want a file lock", path, source)
		}
	}
	for _, path := range []string{"app.search_url", "server.address", "extractors.markdown.enable", "extractors.ytdlp.enable", "extractors.ytdlp.options.extra_args"} {
		if source := c.Source(path); source.Kind != "default" || source.Locked {
			t.Errorf("omitted %s source = %+v, want unlocked defaults", path, source)
		}
	}
	if c.App.Title != "Hister" || c.App.Subtitle != "" || c.App.ShowSuggestions || c.Crawler.Delay != 0 || len(c.Crawler.Headers) != 0 || len(c.Indexer.Languages) != 0 {
		t.Fatal("source tracking changed explicit values")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("candidate loading created runtime files: %v, err=%v", entries, err)
	}
}

func TestConfigurationSourcesEnvironmentAndNormalization(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	const raw = "app:\n  title: From file\n  show_suggestions: true\n  user_handling: false\ncrawler:\n  delay: 10\n  headers:\n    X-Trace: file\nserver:\n  address: 127.0.0.1:4433\n"
	if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"HISTER__APP__TITLE": "", "HISTER__APP__SHOW_SUGGESTIONS": "false",
		"HISTER__CRAWLER__DELAY": "0", "HISTER__CRAWLER__HEADERS__AUTHORIZATION": "private-credential",
		"HISTER__APP__DIRECTORY": filepath.Join(dir, "unused"), "HISTER_DATA_DIR": filepath.Join(dir, "data"),
		"HISTER__SERVER__ADDRESS": "127.0.0.1:5544", "HISTER_PORT": "6655",
		"HISTER__SERVER__PROXY_AUTH_HEADER": "X-Remote-User",
	} {
		t.Setenv(key, value)
	}
	c, _, err := LoadForInspection(filename, true)
	if err != nil {
		t.Fatal(err)
	}
	for path, name := range map[string]string{
		"app.title": "HISTER__APP__TITLE", "app.show_suggestions": "HISTER__APP__SHOW_SUGGESTIONS",
		"crawler.delay": "HISTER__CRAWLER__DELAY", "crawler.headers": "HISTER__CRAWLER__HEADERS__AUTHORIZATION",
		"crawler.headers.x-trace": "HISTER__CRAWLER__HEADERS__AUTHORIZATION",
		"app.directory":           "HISTER_DATA_DIR", "server.address": "HISTER_PORT",
	} {
		if source := c.Source(path); source.Kind != "environment" || source.Name != name || !source.Locked {
			t.Errorf("%s source = %+v, want %s", path, source, name)
		}
	}
	if c.App.Title != "" || c.App.ShowSuggestions || c.Crawler.Delay != 0 || c.Server.Address != "127.0.0.1:6655" || c.App.Directory != filepath.Join(dir, "data") {
		t.Fatal("explicit environment values did not override the file and defaults")
	}
	source := c.Source("app.user_handling")
	if !c.App.UserHandling || !source.Locked || source.DerivedFrom != "server.proxy_auth_header" || source.Name != "HISTER__SERVER__PROXY_AUTH_HEADER" {
		t.Fatalf("proxy authentication dependency was lost: %+v", source)
	}
	if strings.Contains(source.Reason, "X-Remote-User") || strings.Contains(c.Source("crawler.headers").Reason, "private-credential") {
		t.Fatal("source descriptions contain setting values")
	}
}

func TestLoadRequiresRequestedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("LOCALAPPDATA", dir)
	t.Setenv("HISTER_DATA_DIR", filepath.Join(dir, "data"))
	if err := os.MkdirAll(filepath.Join(dir, "hister"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hister", "config.yml"), []byte("app:\n  title: Fallback\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, filename := range []string{filepath.Join(dir, "missing.yml"), dir} {
		if _, err := Load(filename); err == nil {
			t.Errorf("startup silently replaced requested file %q", filename)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "data")); !os.IsNotExist(err) {
		t.Fatalf("failed loading initialized runtime data: %v", err)
	}
}

func TestConfigurationSourcesIgnoreEmptyContainers(t *testing.T) {
	for _, raw := range []string{
		"# app:\n#   title: Hister\n",
		"{}\n",
		"app: {}\nserver:\n  oauth:\n    github: {}\nextractors:\n  ytdlp:\n    options: {}\n",
	} {
		t.Run(raw, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
				t.Fatal(err)
			}
			c, _, err := LoadForInspection(filename, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"app.title", "server.address", "server.oauth.github.client_secret", "extractors.ytdlp.enable", "extractors.ytdlp.options.timeout"} {
				if source := c.Source(path); source.Kind != "default" || source.Locked {
					t.Errorf("empty category pinned %s: %+v", path, source)
				}
			}
		})
	}
}

func TestConfigurationSourcesPreservePartialMaps(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yml")
	const raw = "hotkeys:\n  web:\n    alt+x: focus_search_input\nsensitive_content_patterns:\n  custom: private\n"
	if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	c, _, err := LoadForInspection(filename, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Hotkeys.Web["alt+x"] != "focus_search_input" || c.Hotkeys.Web["enter"] != "open_result" || c.SensitiveContentPatterns["custom"] != "private" || c.SensitiveContentPatterns["github_token"] == "" {
		t.Fatal("source tracking changed how partial main file maps inherit defaults")
	}
	for _, path := range []string{"hotkeys.web", "hotkeys.web.enter", "sensitive_content_patterns", "sensitive_content_patterns.github_token"} {
		if source := c.Source(path); source.Kind != "file" || !source.Locked {
			t.Errorf("authoritative collection member did not lock %s: %+v", path, source)
		}
	}
}

func TestConfigurationSourcesIgnoreInapplicableSpecialEnvironment(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "config.yml")
	address := "unix:" + filepath.Join(dir, "hister.sock")
	raw := fmt.Sprintf("app:\n  directory: %q\nserver:\n  address: %q\n  base_url: http://localhost\n", dir, address)
	if err := os.WriteFile(filename, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTER_DATA_DIR", "")
	t.Setenv("HISTER_PORT", "5544")
	c, _, err := LoadForInspection(filename, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.App.Directory != dir || c.Server.Address != address || c.Source("app.directory").Kind != "file" || c.Source("server.address").Kind != "file" {
		t.Fatal("empty or inapplicable special environment values replaced the file source")
	}
}

func TestConfigurationSourcesPreserveAutomaticEnvironmentPrecedence(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(filename, []byte("app:\n  title: File\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HISTER_APP__TITLE", "Automatic environment")
	for _, explicit := range []bool{false, true} {
		if explicit {
			t.Setenv("HISTER__APP__TITLE", "Explicit environment")
		}
		c, _, err := LoadForInspection(filename, true)
		if err != nil {
			t.Fatal(err)
		}
		name, value := "HISTER_APP__TITLE", "Automatic environment"
		if explicit {
			name, value = "HISTER__APP__TITLE", "Explicit environment"
		}
		if source := c.Source("app.title"); c.App.Title != value || source.Kind != "environment" || source.Name != name {
			t.Errorf("title = %q, source = %+v; want %q from %s", c.App.Title, source, value, name)
		}
	}
}

func TestPDFParserConfiguration(t *testing.T) {
	readParser := func(cfg *Config) map[string]any {
		t.Helper()
		data, err := yaml.Marshal(cfg)
		if err != nil {
			t.Fatal(err)
		}
		var exported struct {
			Indexer struct {
				PDFParser map[string]any `yaml:"pdf_parser"`
			} `yaml:"indexer"`
		}
		if err := yaml.Unmarshal(data, &exported); err != nil {
			t.Fatal(err)
		}
		return exported.Indexer.PDFParser
	}
	defaults := readParser(CreateDefaultConfig())
	if defaults["command"] != "" || defaults["timeout"] != 30 {
		t.Fatalf("default PDF parser = %v, want an empty command and a 30 second timeout", defaults)
	}
	const raw = "indexer:\n  pdf_parser:\n    command: pdftotext\n    args: [-enc, UTF-8, '-', '-']\n    timeout: 15\n"
	cfg, err := parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	parser := readParser(cfg)
	if parser["command"] != "pdftotext" || parser["timeout"] != 15 || len(parser["args"].([]any)) != 4 {
		t.Fatalf("PDF parser config was not loaded: %v", parser)
	}
	t.Setenv("HISTER__INDEXER__PDF_PARSER__COMMAND", "/custom/parser")
	t.Setenv("HISTER__INDEXER__PDF_PARSER__ARGS", "--text,-")
	t.Setenv("HISTER__INDEXER__PDF_PARSER__TIMEOUT", "20")
	cfg, err = parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	parser = readParser(cfg)
	if parser["command"] != "/custom/parser" || parser["timeout"] != 20 || len(parser["args"].([]any)) != 2 {
		t.Fatalf("environment did not override PDF parser config: %v", parser)
	}
}

func TestPDFParserConfigurationRejectsInvalidValues(t *testing.T) {
	for _, raw := range []string{
		"command: '   '",
		"command: ''\n    args: [unused]",
		"timeout: -1",
		"timeout: 0",
		"timeout: 9223372036854775807",
	} {
		t.Run(raw, func(t *testing.T) {
			_, err := parseConfig([]byte("indexer:\n  pdf_parser:\n    " + raw + "\n"))
			if err == nil || !strings.Contains(err.Error(), "pdf_parser") {
				t.Fatalf("error = %v, want PDF parser configuration error", err)
			}
		})
	}
}

func TestAllowAndSkipRules(t *testing.T) {
	for _, tc := range []struct {
		name  string
		allow []string
		skip  []string
		url   string
		want  bool
	}{
		{name: "empty", url: "https://other.example/"},
		{name: "allow first", allow: []string{`^https://one\.example/`, `^https://two\.example/`}, url: "https://one.example/page"},
		{name: "allow second", allow: []string{`^https://one\.example/`, `^https://two\.example/`}, url: "https://two.example/page"},
		{name: "outside allow", allow: []string{`^https://one\.example/`}, url: "https://other.example/", want: true},
		{name: "skip wins", allow: []string{`^https://one\.example/`}, skip: []string{`/private/`}, url: "https://one.example/private/page", want: true},
		{name: "skip only", skip: []string{`/private/`}, url: "https://one.example/private/page", want: true},
		{name: "skip does not match", skip: []string{`/private/`}, url: "https://one.example/public/page"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &Rules{Allow: &Rule{ReStrs: tc.allow}, Skip: &Rule{ReStrs: tc.skip}}
			if err := r.Compile(); err != nil {
				t.Fatal(err)
			}
			if got := r.IsSkip(tc.url); got != tc.want {
				t.Errorf("IsSkip(%q) = %v, want %v", tc.url, got, tc.want)
			}
			if got := r.Count(); got != len(tc.allow)+len(tc.skip) {
				t.Errorf("Count() = %d", got)
			}
		})
	}
	r := &Rules{Allow: &Rule{ReStrs: []string{"["}}}
	if err := r.Compile(); err == nil {
		t.Fatal("invalid allow pattern was accepted")
	}
	if (*Rules)(nil).IsSkip("https://example.com/") || (&Rules{}).IsSkip("https://example.com/") {
		t.Fatal("missing rules should permit URLs")
	}
}

func TestAllowRulesLoadAndSave(t *testing.T) {
	for _, content := range []string{`{}`, `{"allow":null}`, `{"skip":["private"]}`, `{"allow":["example.com"]}`} {
		t.Run(content, func(t *testing.T) {
			cfg := &Config{App: App{Directory: t.TempDir()}}
			if err := os.WriteFile(cfg.RulesPath(), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := cfg.LoadRules(); err != nil {
				t.Fatal(err)
			}
			if cfg.Rules.Allow == nil || cfg.Rules.Allow.ReStrs == nil {
				t.Fatal("missing allow rules were not initialized")
			}
			cfg.Rules.Allow.ReStrs = []string{`^https://allowed\.example/`}
			if err := cfg.SaveRules(); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(cfg.RulesPath())
			if err != nil {
				t.Fatal(err)
			}
			var saved Rules
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Allow == nil || saved.IsSkip("https://allowed.example/page") || !saved.IsSkip("https://other.example/page") {
				t.Fatal("saved allow rules were not preserved")
			}
		})
	}
}

func restoreEnv(key, value string, existed bool) {
	if existed {
		_ = os.Setenv(key, value)
		return
	}
	_ = os.Unsetenv(key)
}

func TestFullPathPreservesAbsolutePath(t *testing.T) {
	absolutePath := t.TempDir()
	cfg := Config{App: App{Directory: t.TempDir()}}

	if got := cfg.FullPath(absolutePath); got != absolutePath {
		t.Fatalf("FullPath(%q) = %q, want absolute path unchanged", absolutePath, got)
	}
}

func TestServerDefaults(t *testing.T) {
	oldAddress := DefaultServerAddress
	oldBaseURL := DefaultServerBaseURL
	t.Cleanup(func() {
		DefaultServerAddress = oldAddress
		DefaultServerBaseURL = oldBaseURL
	})

	DefaultServerAddress = "127.0.0.1:5544"
	DefaultServerBaseURL = "https://defaults.example.com"

	cfg := CreateDefaultConfig()
	if cfg.Server.Address != DefaultServerAddress {
		t.Fatalf("default server address=%q, want %q", cfg.Server.Address, DefaultServerAddress)
	}
	if cfg.Server.BaseURL != DefaultServerBaseURL {
		t.Fatalf("default server base_url=%q, want %q", cfg.Server.BaseURL, DefaultServerBaseURL)
	}
	if cfg.Server.MaxBatchBodySize != DefaultMaxBatchBodySize {
		t.Fatalf("default server max_batch_body_size=%d, want %d", cfg.Server.MaxBatchBodySize, DefaultMaxBatchBodySize)
	}
	if cfg.Server.MaxBatchBodyBytes() != 40<<20 {
		t.Fatalf("default server batch body bytes=%d, want %d", cfg.Server.MaxBatchBodyBytes(), 40<<20)
	}
}

func TestDefaultConfigYAMLOmitsTUIHotkeys(t *testing.T) {
	b, err := yaml.Marshal(CreateDefaultConfig())
	if err != nil {
		t.Fatal(err)
	}

	var generated struct {
		Hotkeys map[string]any `yaml:"hotkeys"`
	}
	if err := yaml.Unmarshal(b, &generated); err != nil {
		t.Fatal(err)
	}
	if _, ok := generated.Hotkeys["web"]; !ok {
		t.Fatal("generated config does not contain web hotkeys")
	}
	if _, ok := generated.Hotkeys["tui"]; ok {
		t.Fatal("generated config contains TUI hotkeys")
	}
}

func TestShowSuggestionsConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		env  string
		want bool
	}{
		{name: "default", want: false},
		{name: "enabled", raw: "app:\n  show_suggestions: true\n", want: true},
		{name: "disabled", raw: "app:\n  show_suggestions: false\n"},
		{name: "environment disables", raw: "app:\n  show_suggestions: true\n", env: "false"},
		{name: "environment enables", raw: "app:\n  show_suggestions: false\n", env: "true", want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("HISTER__APP__SHOW_SUGGESTIONS", tc.env)
			}
			cfg, err := parseConfig([]byte(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			data, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatal(err)
			}
			var exported struct {
				App map[string]any `yaml:"app"`
			}
			if err := yaml.Unmarshal(data, &exported); err != nil {
				t.Fatal(err)
			}
			if got := exported.App["show_suggestions"]; got != tc.want {
				t.Fatalf("generated app.show_suggestions = %v, want %v", got, tc.want)
			}
		})
	}
	if _, err := parseConfig([]byte("app:\n  show_suggestions: invalid\n")); err == nil {
		t.Fatal("invalid app.show_suggestions was accepted")
	}
}

func TestServerMaxBatchBodySizeConfig(t *testing.T) {
	cfg, err := parseConfig([]byte("server:\n  max_batch_body_size: 12\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.MaxBatchBodyBytes() != 12<<20 {
		t.Fatalf("server batch body bytes=%d, want %d", cfg.Server.MaxBatchBodyBytes(), 12<<20)
	}

	if _, err := parseConfig([]byte("server:\n  max_batch_body_size: 0\n")); err == nil {
		t.Fatal("zero server.max_batch_body_size was accepted")
	}
}

func TestServerMaxBatchBodySizeEnvironmentOverride(t *testing.T) {
	const envName = "HISTER__SERVER__MAX_BATCH_BODY_SIZE"
	oldValue, existed := os.LookupEnv(envName)
	t.Cleanup(func() { restoreEnv(envName, oldValue, existed) })
	if err := os.Setenv(envName, "24"); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig([]byte("server:\n  max_batch_body_size: 12\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.MaxBatchBodySize != 24 {
		t.Fatalf("server.max_batch_body_size=%d, want environment value 24", cfg.Server.MaxBatchBodySize)
	}
}

func TestIndexerDefaults(t *testing.T) {
	cfg := CreateDefaultConfig()
	if !cfg.Indexer.DetectLanguages {
		t.Fatal("default indexer.detect_languages=false, want true")
	}
	if cfg.Indexer.KeepStopwords {
		t.Fatal("default indexer.keep_stopwords=true, want false")
	}

	cfg, err := parseConfig([]byte("indexer:\n  keep_stopwords: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Indexer.KeepStopwords {
		t.Fatal("configured indexer.keep_stopwords=false, want true")
	}
}

func TestIndexerValidate(t *testing.T) {
	for _, tc := range []struct {
		name    string
		indexer Indexer
		wantErr bool
	}{
		{"no languages detects everything", Indexer{}, false},
		{"two languages", Indexer{Languages: []string{"en", "de"}}, false},
		{"one language", Indexer{Languages: []string{"en"}}, true},
		// A repeated code resolves to a single language, which would leave the
		// detector disabled rather than narrowed.
		{"repeated language", Indexer{Languages: []string{"en", " EN "}}, true},
		{"regional duplicates", Indexer{Languages: []string{"en-US", "en-GB"}}, true},
		{"norwegian aliases", Indexer{Languages: []string{"no", "nb"}}, true},
		{"regional candidates", Indexer{Languages: []string{"en-US", "de-DE"}}, false},
		{"unsupported detector", Indexer{Languages: []string{"en", "uk"}}, true},
		{"unknown accuracy", Indexer{LanguageDetectionAccuracy: "medium"}, true},
		{"low accuracy", Indexer{LanguageDetectionAccuracy: "low"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.indexer.Validate(); (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestIndexerLanguageConfiguration(t *testing.T) {
	generated, err := yaml.Marshal(CreateDefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	var defaults struct {
		Indexer map[string]any `yaml:"indexer"`
	}
	if err := yaml.Unmarshal(generated, &defaults); err != nil {
		t.Fatal(err)
	}
	if _, exists := defaults.Indexer["languages"]; !exists {
		t.Fatal("generated config is missing indexer.languages")
	}
	if got := defaults.Indexer["language_detection_accuracy"]; got != "low" {
		t.Fatalf("generated accuracy = %v, want low", got)
	}
	const raw = "indexer:\n  languages: [en, de]\n  language_detection_accuracy: low\n"
	cfg, err := parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Indexer.Languages, []string{"en", "de"}) || !cfg.Indexer.LowAccuracyLanguageDetection() {
		t.Fatalf("language settings did not load from YAML: %+v", cfg.Indexer)
	}
	t.Setenv("HISTER__INDEXER__LANGUAGES", "fr,no")
	t.Setenv("HISTER__INDEXER__LANGUAGE_DETECTION_ACCURACY", "high")
	cfg, err = parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.Indexer.Languages, []string{"fr", "no"}) || cfg.Indexer.LowAccuracyLanguageDetection() {
		t.Fatalf("environment did not override language settings: %+v", cfg.Indexer)
	}
}

func TestLanguageDetectionAccuracyDefaultsAndOverrides(t *testing.T) {
	for _, tc := range []struct {
		name string
		yaml string
		low  bool
	}{
		{"omitted", "{}", true},
		{"empty", "indexer:\n  language_detection_accuracy: ''\n", true},
		{"explicit low", "indexer:\n  language_detection_accuracy: low\n", true},
		{"explicit high", "indexer:\n  language_detection_accuracy: high\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseConfig([]byte(tc.yaml))
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.Indexer.LowAccuracyLanguageDetection(); got != tc.low {
				t.Errorf("LowAccuracyLanguageDetection = %v, want %v", got, tc.low)
			}
		})
	}
}

func TestDirectoryLabelConfig(t *testing.T) {
	cfg, err := parseConfig([]byte("indexer:\n  directories:\n    - path: /srv/docs\n      label: reference\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Indexer.Directories) != 1 {
		t.Fatalf("directory count=%d, want 1", len(cfg.Indexer.Directories))
	}
	if got := cfg.Indexer.Directories[0].Label; got != "reference" {
		t.Fatalf("directory label=%q, want %q", got, "reference")
	}
}

func TestSemanticSearchRequestDefaults(t *testing.T) {
	cfg, err := parseConfig([]byte("semantic_search:\n  max_context_length: 256\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.SemanticSearch.EmbeddingTimeout, 300; got != want {
		t.Errorf("semantic_search.embedding_timeout = %d, want %d", got, want)
	}
	if got, want := cfg.SemanticSearch.MaxEmbeddingBatchSize, 8; got != want {
		t.Errorf("semantic_search.max_embedding_batch_size = %d, want %d", got, want)
	}
	if got, want := cfg.SemanticSearch.MaxEmbeddingConcurrency, 2; got != want {
		t.Errorf("semantic_search.max_embedding_concurrency = %d, want %d", got, want)
	}
	if got, want := cfg.SemanticSearch.QueryEmbeddingTimeout, 2; got != want {
		t.Errorf("query embedding timeout = %d, want %d", got, want)
	}
	if got, want := cfg.SemanticSearch.MaxQueryEmbeddingConcurrency, 1; got != want {
		t.Errorf("query embedding concurrency = %d, want %d", got, want)
	}

	if got, want := cfg.SemanticSearch.Dimensions, 2000; got != want {
		t.Errorf("semantic_search.dimensions = %d, want %d", got, want)
	}
}

func TestSemanticSearchPostgresDimensions(t *testing.T) {
	tests := []struct {
		name       string
		database   string
		enable     bool
		dimensions int
		wantError  string
	}{
		{
			name:       "PostgreSQL maximum",
			database:   "host=localhost dbname=hister",
			enable:     true,
			dimensions: 2000,
		},
		{
			name:       "PostgreSQL above maximum",
			database:   "host=localhost dbname=hister",
			enable:     true,
			dimensions: 2001,
			wantError:  "semantic_search.dimensions must not exceed 2000 when using PostgreSQL, got 2001",
		},
		{
			name:       "SQLite above PostgreSQL maximum",
			database:   "db.sqlite3",
			enable:     true,
			dimensions: 4096,
		},
		{
			name:       "disabled PostgreSQL semantic search",
			database:   "host=localhost dbname=hister",
			dimensions: 4096,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := CreateDefaultConfig()
			cfg.Server.Database = test.database
			cfg.SemanticSearch.Enable = test.enable
			cfg.SemanticSearch.Dimensions = test.dimensions

			err := cfg.validateSemanticSearch()
			if test.wantError == "" {
				if err != nil {
					t.Fatalf("validateSemanticSearch() error = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != test.wantError {
				t.Fatalf("validateSemanticSearch() error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func TestCrawlerProxyConfigAndEnvironment(t *testing.T) {
	const envName = "HISTER__CRAWLER__PROXY"
	oldProxy, hadProxy := os.LookupEnv(envName)
	t.Cleanup(func() {
		restoreEnv(envName, oldProxy, hadProxy)
	})
	if err := os.Unsetenv(envName); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig([]byte("crawler:\n  proxy: http://file-proxy.example:8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Crawler.Proxy, "http://file-proxy.example:8080"; got != want {
		t.Fatalf("crawler proxy = %q, want %q", got, want)
	}

	if err := os.Setenv(envName, "socks5://env-proxy.example:1080"); err != nil {
		t.Fatal(err)
	}
	cfg, err = parseConfig([]byte("crawler:\n  proxy: http://file-proxy.example:8080\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.Crawler.Proxy, "socks5://env-proxy.example:1080"; got != want {
		t.Fatalf("crawler proxy = %q, want %q", got, want)
	}
}

func TestAppTitleDefaultsAndOverrides(t *testing.T) {
	cfg := CreateDefaultConfig()
	if cfg.App.Title != "Hister" {
		t.Fatalf("default app title=%q, want %q", cfg.App.Title, "Hister")
	}
	if cfg.App.Subtitle != "Your own search engine" {
		t.Fatalf("default app subtitle=%q, want %q", cfg.App.Subtitle, "Your own search engine")
	}

	cfg, err := parseConfig([]byte("app:\n  title: Team Archive\n  subtitle: Internal search\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.App.Title != "Team Archive" {
		t.Fatalf("app title=%q, want %q", cfg.App.Title, "Team Archive")
	}
	if cfg.App.Subtitle != "Internal search" {
		t.Fatalf("app subtitle=%q, want %q", cfg.App.Subtitle, "Internal search")
	}
}

func TestAppColorScheme(t *testing.T) {
	if got := CreateDefaultConfig().App.ColorScheme; got != "automatic" {
		t.Fatalf("default app color_scheme=%q, want %q", got, "automatic")
	}

	for _, scheme := range []string{"automatic", "dark", "light"} {
		t.Run(scheme, func(t *testing.T) {
			cfg, err := parseConfig([]byte("app:\n  color_scheme: " + scheme + "\n"))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.App.ColorScheme != scheme {
				t.Fatalf("app color_scheme=%q, want %q", cfg.App.ColorScheme, scheme)
			}
		})
	}

	if _, err := parseConfig([]byte("app:\n  color_scheme: sepia\n")); err == nil {
		t.Fatal("invalid app.color_scheme was accepted")
	}
}

func TestConfigFileOverridesServerDefaults(t *testing.T) {
	oldAddress := DefaultServerAddress
	oldBaseURL := DefaultServerBaseURL
	oldEnvAddress, hadEnvAddress := os.LookupEnv("HISTER__SERVER__ADDRESS")
	oldEnvBaseURL, hadEnvBaseURL := os.LookupEnv("HISTER__SERVER__BASE_URL")
	t.Cleanup(func() {
		DefaultServerAddress = oldAddress
		DefaultServerBaseURL = oldBaseURL
		restoreEnv("HISTER__SERVER__ADDRESS", oldEnvAddress, hadEnvAddress)
		restoreEnv("HISTER__SERVER__BASE_URL", oldEnvBaseURL, hadEnvBaseURL)
	})

	DefaultServerAddress = "127.0.0.1:4433"
	DefaultServerBaseURL = "http://defaults.example.com"
	_ = os.Unsetenv("HISTER__SERVER__ADDRESS")
	_ = os.Unsetenv("HISTER__SERVER__BASE_URL")

	cfg, err := parseConfig([]byte("server:\n  address: 0.0.0.0:9999\n  base_url: https://config.example.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != "0.0.0.0:9999" {
		t.Fatalf("server address=%q, want config file value %q", cfg.Server.Address, "0.0.0.0:9999")
	}
	if cfg.Server.BaseURL != "https://config.example.com" {
		t.Fatalf("server base_url=%q, want config file value %q", cfg.Server.BaseURL, "https://config.example.com")
	}
}

func TestEnvironmentOverridesConfigFile(t *testing.T) {
	oldEnvAddress, hadEnvAddress := os.LookupEnv("HISTER__SERVER__ADDRESS")
	oldEnvBaseURL, hadEnvBaseURL := os.LookupEnv("HISTER__SERVER__BASE_URL")
	t.Cleanup(func() {
		restoreEnv("HISTER__SERVER__ADDRESS", oldEnvAddress, hadEnvAddress)
		restoreEnv("HISTER__SERVER__BASE_URL", oldEnvBaseURL, hadEnvBaseURL)
	})

	if err := os.Setenv("HISTER__SERVER__ADDRESS", "0.0.0.0:9999"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("HISTER__SERVER__BASE_URL", "https://env.example.com"); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig([]byte("server:\n  address: 127.0.0.1:4433\n  base_url: https://config.example.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != "0.0.0.0:9999" {
		t.Fatalf("server address=%q, want environment value %q", cfg.Server.Address, "0.0.0.0:9999")
	}
	if cfg.Server.BaseURL != "https://env.example.com" {
		t.Fatalf("server base_url=%q, want environment value %q", cfg.Server.BaseURL, "https://env.example.com")
	}
}

func TestCLIFlagsOverrideEnvironment(t *testing.T) {
	oldEnvAddress, hadEnvAddress := os.LookupEnv("HISTER__SERVER__ADDRESS")
	oldEnvBaseURL, hadEnvBaseURL := os.LookupEnv("HISTER__SERVER__BASE_URL")
	t.Cleanup(func() {
		restoreEnv("HISTER__SERVER__ADDRESS", oldEnvAddress, hadEnvAddress)
		restoreEnv("HISTER__SERVER__BASE_URL", oldEnvBaseURL, hadEnvBaseURL)
	})

	if err := os.Setenv("HISTER__SERVER__ADDRESS", "0.0.0.0:9999"); err != nil {
		t.Fatal(err)
	}
	if err := os.Setenv("HISTER__SERVER__BASE_URL", "https://env.example.com"); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig([]byte("server:\n  address: 127.0.0.1:4433\n  base_url: https://config.example.com\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Address != "0.0.0.0:9999" {
		t.Fatalf("precondition: server address=%q, want environment value %q", cfg.Server.Address, "0.0.0.0:9999")
	}
	if cfg.Server.BaseURL != "https://env.example.com" {
		t.Fatalf("precondition: server base_url=%q, want environment value %q", cfg.Server.BaseURL, "https://env.example.com")
	}

	if err := cfg.UpdateBaseURL("https://cli.example.com"); err != nil {
		t.Fatal(err)
	}
	if err := cfg.UpdateListenAddress("127.0.0.1:7777"); err != nil {
		t.Fatal(err)
	}

	if cfg.Server.Address != "127.0.0.1:7777" {
		t.Fatalf("server address=%q, want CLI flag value %q", cfg.Server.Address, "127.0.0.1:7777")
	}
	if cfg.Server.BaseURL != "https://cli.example.com" {
		t.Fatalf("server base_url=%q, want CLI flag value %q", cfg.Server.BaseURL, "https://cli.example.com")
	}
}

func TestBasePathPrefix(t *testing.T) {
	tests := []struct {
		name   string
		base   string
		prefix string
	}{
		{name: "root-no-slash", base: "https://example.com", prefix: ""},
		{name: "root-with-slash", base: "https://example.com/", prefix: ""},
		{name: "subfolder", base: "https://example.com/subfolder", prefix: "/subfolder"},
		{name: "subfolder-trailing", base: "https://example.com/subfolder/", prefix: "/subfolder"},
		{name: "nested", base: "https://example.com/a/b", prefix: "/a/b"},
		{name: "nested-trailing", base: "https://example.com/a/b/", prefix: "/a/b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Server: Server{BaseURL: tt.base}}
			if got := cfg.BasePathPrefix(); got != tt.prefix {
				t.Fatalf("BasePathPrefix()=%q, want %q", got, tt.prefix)
			}
		})
	}
}

func TestSensitiveContentPatterns(t *testing.T) {
	patterns := CreateDefaultConfig().SensitiveContentPatterns
	tests := []struct {
		name    string
		pattern string
		input   string
		match   bool
	}{
		{name: "aws_access_key/quoted", pattern: "aws_access_key", input: `key: "AKIAIOSFODNN7EXAMPLE"`, match: true},
		{name: "aws_access_key/whitespace", pattern: "aws_access_key", input: "token AKIAIOSFODNN7EXAMPLE end", match: true},
		{name: "aws_access_key/single-quoted", pattern: "aws_access_key", input: `'AKIAIOSFODNN7EXAMPLE'`, match: true},
		{name: "aws_access_key/start-of-string", pattern: "aws_access_key", input: "AKIAIOSFODNN7EXAMPLE ", match: true},
		{name: "aws_access_key/end-of-string", pattern: "aws_access_key", input: " AKIAIOSFODNN7EXAMPLE", match: true},
		{name: "aws_access_key/base64-blob", pattern: "aws_access_key", input: "d09GMgABAAAAAKIAIOSFODNN7EXAMPLEXYZABCDEF", match: false},
		{name: "aws_access_key/css-font", pattern: "aws_access_key", input: "url(data:font/woff2;base64,d09GMgABAAAAAKIA1234567890ABCDEF)", match: false},
		{name: "github_token/valid", pattern: "github_token", input: "ghp_abcdefghijklmnopqrstuvwxyzABCDEFGHIJ", match: true},
		{name: "generic_private_key", pattern: "generic_private_key", input: "-----BEGIN RSA PRIVATE KEY-----", match: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, ok := patterns[tt.pattern]
			if !ok {
				t.Fatalf("pattern %q not in defaults", tt.pattern)
			}
			re := regexp.MustCompile(raw)
			if got := re.MatchString(tt.input); got != tt.match {
				t.Fatalf("MatchString(%q) = %v, want %v", tt.input, got, tt.match)
			}
		})
	}
}

func TestParseEnvValue(t *testing.T) {
	tests := []struct {
		in   string
		want any
	}{
		{"true", true},
		{"false", false},
		{"True", true},
		{"FALSE", false},
		{"15", 15},
		{"0", 0},
		{"-3", -3},
		{"1.5", 1.5},
		// stays string: not a bool keyword
		{"yes", "yes"},
		{"1", 1},
		// stays string: numeric round-trip fails (data-loss guard)
		{"007", "007"},
		{"+5", "+5"},
		{"1e3", "1e3"},
		{"1.50", "1.50"},
		// plain strings
		{"en", "en"},
		{"https://env.example.com", "https://env.example.com"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := parseEnvValue(tt.in)
			if got != tt.want {
				t.Fatalf("parseEnvValue(%q) = %#v (%T), want %#v (%T)", tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEnvExtractorOptionTypes(t *testing.T) {
	const (
		enableKey = "HISTER__EXTRACTORS__ytdlp__ENABLE"
		subsKey   = "HISTER__EXTRACTORS__ytdlp__OPTIONS__FETCH_SUBTITLES"
		toKey     = "HISTER__EXTRACTORS__ytdlp__OPTIONS__TIMEOUT"
		jobsKey   = "HISTER__EXTRACTORS__ytdlp__OPTIONS__MAX_CONCURRENT_JOBS"
		langKey   = "HISTER__EXTRACTORS__ytdlp__OPTIONS__SUB_LANGUAGE"
	)
	for _, k := range []string{enableKey, subsKey, toKey, jobsKey, langKey} {
		old, had := os.LookupEnv(k)
		t.Cleanup(func() { restoreEnv(k, old, had) })
	}
	_ = os.Setenv(enableKey, "true")
	_ = os.Setenv(subsKey, "true")
	_ = os.Setenv(toKey, "30")
	_ = os.Setenv(jobsKey, "2")
	_ = os.Setenv(langKey, "en")

	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	ex := cfg.Extractors["ytdlp"]
	if ex == nil {
		t.Fatalf("ytdlp extractor missing; extractors=%v", cfg.Extractors)
	}
	if !ex.Enable {
		t.Errorf("Enable = false, want true")
	}
	if v, ok := ex.Options["fetch_subtitles"].(bool); !ok || !v {
		t.Errorf("fetch_subtitles = %#v (%T), want bool true", ex.Options["fetch_subtitles"], ex.Options["fetch_subtitles"])
	}
	if v, ok := ex.Options["timeout"].(int); !ok || v != 30 {
		t.Errorf("timeout = %#v (%T), want int 30", ex.Options["timeout"], ex.Options["timeout"])
	}
	if v, ok := ex.Options["max_concurrent_jobs"].(int); !ok || v != 2 {
		t.Errorf("max_concurrent_jobs = %#v (%T), want int 2", ex.Options["max_concurrent_jobs"], ex.Options["max_concurrent_jobs"])
	}
	if v, ok := ex.Options["sub_language"].(string); !ok || v != "en" {
		t.Errorf("sub_language = %#v (%T), want string \"en\"", ex.Options["sub_language"], ex.Options["sub_language"])
	}
}

func TestEnvTypedStringFieldNotCoerced(t *testing.T) {
	const key = "HISTER__APP__ACCESS_TOKEN"
	old, had := os.LookupEnv(key)
	t.Cleanup(func() { restoreEnv(key, old, had) })

	for _, token := range []string{"True", "false", "5", "0123"} {
		_ = os.Setenv(key, token)
		cfg, err := parseConfig(nil)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.App.AccessToken != token {
			t.Errorf("AccessToken = %q, want verbatim %q", cfg.App.AccessToken, token)
		}
	}
}

func TestPublicModeConfig(t *testing.T) {
	cfg := CreateDefaultConfig()
	if cfg.App.Public {
		t.Fatal("Public default = true, want false")
	}

	cfg, err := parseConfig([]byte("app:\n  public: true\n  access_token: secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.App.Public {
		t.Fatal("Public = false, want true")
	}
	if err := cfg.ValidatePublicMode(); err != nil {
		t.Fatalf("ValidatePublicMode() error = %v, want nil", err)
	}
}

func TestPublicModeEnvironmentOverride(t *testing.T) {
	const key = "HISTER__APP__PUBLIC"
	old, had := os.LookupEnv(key)
	t.Cleanup(func() { restoreEnv(key, old, had) })

	if err := os.Setenv(key, "true"); err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig([]byte("app:\n  access_token: secret\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.App.Public {
		t.Fatal("Public = false, want true")
	}
}

func TestPublicModeRequiresAuth(t *testing.T) {
	cfg := CreateDefaultConfig()
	cfg.App.Public = true
	cfg.App.AccessToken = ""
	cfg.App.UserHandling = false
	if err := cfg.ValidatePublicMode(); err == nil {
		t.Fatal("ValidatePublicMode() error = nil, want error")
	}

	cfg.App.AccessToken = "secret"
	if err := cfg.ValidatePublicMode(); err != nil {
		t.Fatalf("ValidatePublicMode() with access token error = %v, want nil", err)
	}

	cfg.App.AccessToken = ""
	cfg.App.UserHandling = true
	if err := cfg.ValidatePublicMode(); err != nil {
		t.Fatalf("ValidatePublicMode() with user handling error = %v, want nil", err)
	}
}

func TestWebSocketURLHonorsBasePath(t *testing.T) {
	tests := []struct {
		name string
		base string
		want string
	}{
		{name: "http-root", base: "http://example.com:1234", want: "ws://example.com:1234/search"},
		{name: "https-root", base: "https://example.com", want: "wss://example.com/search"},
		{name: "http-subfolder", base: "http://example.com/subfolder", want: "ws://example.com/subfolder/search"},
		{name: "https-nested", base: "https://example.com/a/b/", want: "wss://example.com/a/b/search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Server: Server{BaseURL: tt.base}}
			if got := cfg.WebSocketURL(); got != tt.want {
				t.Fatalf("WebSocketURL()=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestMergeDefaultTUIHotkeysPreservesCustomBindings(t *testing.T) {
	hotkeys := map[string]string{
		"x": string(ActionCopyResult),
		"y": string(ActionOpenResult),
	}
	mergeDefaultTUIHotkeys(hotkeys)

	if got := hotkeys["x"]; got != string(ActionCopyResult) {
		t.Fatalf("custom copy binding = %q", got)
	}
	if got := hotkeys["y"]; got != string(ActionOpenResult) {
		t.Fatalf("occupied default key was overwritten with %q", got)
	}
	if got := hotkeys["ctrl+e"]; got != string(ActionToggleSemantic) {
		t.Fatalf("new semantic binding = %q, want %q", got, ActionToggleSemantic)
	}
}

func TestDefaultTUIUsesTerminalAppearance(t *testing.T) {
	if got := DefaultTUIConfig.ColorScheme; got != "terminal" {
		t.Fatalf("default TUI color scheme = %q, want terminal", got)
	}
}

func TestSemanticSearchEmbeddingFingerprint(t *testing.T) {
	baseConfig := SemanticSearch{
		Enable:            true,
		EmbeddingEndpoint: "http://localhost:11434/v1/embeddings",
		EmbeddingModel:    "model",
		Dimensions:        768,
		MaxContextLength:  512,
		ChunkOverlap:      64,
		DocumentPrefix:    "document: ",
	}
	base := baseConfig.EmbeddingFingerprint()
	if base == "" {
		t.Fatal("enabled semantic search must have an embedding fingerprint")
	}
	if base != baseConfig.EmbeddingFingerprint() {
		t.Fatal("embedding fingerprint must be deterministic")
	}

	changingFields := []func(*SemanticSearch){
		func(cfg *SemanticSearch) { cfg.EmbeddingEndpoint = "http://example.com/v1/embeddings" },
		func(cfg *SemanticSearch) { cfg.EmbeddingModel = "other-model" },
		func(cfg *SemanticSearch) { cfg.Dimensions = 1024 },
		func(cfg *SemanticSearch) { cfg.MaxContextLength = 1024 },
		func(cfg *SemanticSearch) { cfg.ChunkOverlap = 128 },
		func(cfg *SemanticSearch) { cfg.DocumentPrefix = "passage: " },
	}
	for i, change := range changingFields {
		changed := baseConfig
		change(&changed)
		if base == changed.EmbeddingFingerprint() {
			t.Errorf("stored embedding field change %d did not affect fingerprint", i)
		}
	}

	queryConfig := baseConfig
	queryConfig.QueryPrefix = "query: "
	queryConfig.SimilarityThreshold = 0.9
	queryConfig.ResultLimit = 5
	queryConfig.SemanticWeight = 0.8
	queryConfig.EmbeddingTimeout = 30
	queryConfig.MaxEmbeddingBatchSize = 1
	queryConfig.MaxEmbeddingConcurrency = 1
	queryConfig.MaxQueryEmbeddingConcurrency = 3
	queryConfig.QueryEmbeddingTimeout = 10
	if base != queryConfig.EmbeddingFingerprint() {
		t.Fatal("query and operational settings must not affect stored embedding fingerprint")
	}

	disabledConfig := baseConfig
	disabledConfig.Enable = false
	if fingerprint := disabledConfig.EmbeddingFingerprint(); fingerprint != "" {
		t.Fatalf("disabled semantic search fingerprint = %q, want empty", fingerprint)
	}
}

func TestSemanticQuerySettingsOverrides(t *testing.T) {
	const raw = "semantic_search:\n  query_embedding_timeout: 9\n  max_query_embedding_concurrency: 3\n"
	cfg, err := parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SemanticSearch.QueryEmbeddingTimeout != 9 || cfg.SemanticSearch.MaxQueryEmbeddingConcurrency != 3 {
		t.Fatal("query settings did not load from YAML")
	}
	t.Setenv("HISTER__SEMANTIC_SEARCH__QUERY_EMBEDDING_TIMEOUT", "12")
	t.Setenv("HISTER__SEMANTIC_SEARCH__MAX_QUERY_EMBEDDING_CONCURRENCY", "4")
	cfg, err = parseConfig([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SemanticSearch.QueryEmbeddingTimeout != 12 || cfg.SemanticSearch.MaxQueryEmbeddingConcurrency != 4 {
		t.Fatal("environment did not override query settings")
	}
}
