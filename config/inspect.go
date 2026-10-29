// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"gopkg.in/yaml.v3"
)

// ResolvePath uses the normal configuration search order. Explicit paths must
// exist. An empty result means defaults and environment variables are in use.
// Unreadable files are errors rather than permission to use another source.
func ResolvePath(filename string, explicit bool) (string, error) {
	paths := []string{filename}
	if !explicit {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		paths = append(paths, getConfigSearchPaths(home)...)
	}
	for _, name := range paths {
		info, err := os.Stat(name)
		if !explicit && errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("inspect config file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return "", fmt.Errorf("config path %q is not a regular file", name)
		}
		return filepath.Abs(name)
	}
	return "", nil
}

// LoadForInspection decodes defaults, YAML, environment variables, and explicit
// overrides without creating runtime files or opening databases. Call Validate
// on the result. Rules and the separate TUI config are not loaded.
func LoadForInspection(filename string, explicit bool, overrides ...Override) (*Config, string, error) {
	name, err := ResolvePath(filename, explicit)
	if err != nil {
		return nil, name, err
	}
	c, err := loadSelected(name, true, overrides...)
	return c, name, err
}

// loadSelected composes a detached candidate without discovery or runtime IO.
// An empty name deliberately retains the absence of a main configuration file.
func loadSelected(name string, strict bool, overrides ...Override) (*Config, error) {
	var raw []byte
	if name != "" {
		var err error
		raw, err = os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read config file: %w", err)
		}
	}
	inputs, err := loadInputs(raw, name)
	if err != nil {
		return nil, err
	}
	c, err := inputs.decode(strict)
	if err != nil {
		return nil, err
	}
	environment, err := c.environmentOverrides()
	if err != nil {
		return nil, err
	}
	if len(environment)+len(overrides) > 0 {
		if err := inputs.apply(append(environment, overrides...)...); err != nil {
			return nil, err
		}
		c, err = inputs.decode(strict)
		if err != nil {
			return nil, err
		}
	}
	c.fname = name
	return c, c.normalize()
}

func decodeConfig(raw []byte, name string, strict bool) (*Config, error) {
	inputs, err := loadInputs(raw, name)
	if err != nil {
		return nil, err
	}
	return inputs.decode(strict)
}

func (inputs *configInputs) decode(strict bool) (*Config, error) {
	c := CreateDefaultConfig()
	c.sources = maps.Clone(inputs.sources)
	var metadata mapstructure.Metadata
	if err := inputs.values.Unmarshal(c, func(dc *mapstructure.DecoderConfig) { dc.Metadata = &metadata }); err != nil {
		// Decoder errors can include credential values. Do not echo them.
		if field, ok := errors.AsType[*mapstructure.DecodeError](err); ok {
			return nil, fmt.Errorf("invalid configuration value at %s", field.Name())
		}
		return nil, errors.New("invalid configuration keys or value types")
	}
	if strict && len(metadata.Unused) > 0 {
		slices.Sort(metadata.Unused)
		return nil, fmt.Errorf("unknown configuration keys: %s", strings.Join(metadata.Unused, ", "))
	}
	return c, nil
}

// Validate checks the main configuration without initializing runtime state.
func (c *Config) Validate() error {
	if err := c.validateBasic(); err != nil {
		return err
	}
	if err := c.UpdateBaseURL(c.Server.BaseURL); err != nil {
		return errors.New("invalid server.address or server.base_url; set an explicit base URL when listening on all interfaces or a Unix socket")
	}
	u := c.parsedBaseURL
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("server.base_url must be an HTTP or HTTPS URL without a query or fragment")
	}
	if err := c.ValidatePublicMode(); err != nil {
		return err
	}
	if err := c.Hotkeys.Validate(); err != nil {
		return err
	}
	if err := c.validateSemanticSearch(); err != nil {
		return err
	}
	if err := c.Indexer.Validate(); err != nil {
		return err
	}
	if err := c.validateOAuth(); err != nil {
		return err
	}
	switch c.App.LogLevel {
	case "", "error", "err", "warning", "warn", "info", "debug", "trace":
	default:
		return errors.New("app.log_level must be error, warning, info, debug, or trace")
	}
	switch c.App.LogFormat {
	case "", "text", "json":
	default:
		return errors.New("app.log_format must be text or json")
	}
	switch c.Crawler.Backend {
	case "", "http", "chromedp", "bidi":
	default:
		return errors.New("crawler.backend must be http, chromedp, or bidi")
	}
	return nil
}

const redactedValue = "[REDACTED]"

// Redacted returns a detached YAML representation for diagnostic output.
func (c *Config) Redacted() (map[string]any, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := yaml.Unmarshal(b, &result); err != nil {
		return nil, err
	}
	redactMap(result)
	return result, nil
}

func redactMap(values map[string]any) {
	for key, value := range values {
		k := strings.ToLower(key)
		if k == "sensitive_content_patterns" {
			continue
		}
		if strings.Contains(k, "secret") || strings.Contains(k, "password") || k == "token" || strings.HasSuffix(k, "_token") || k == "api_key" || k == "authorization" || k == "extra_args" || k == "args" {
			if value != nil && value != "" {
				values[key] = redactedValue
			}
			continue
		}
		switch v := value.(type) {
		case map[string]any:
			if k == "headers" {
				for header := range v {
					v[header] = redactedValue
				}
			} else {
				redactMap(v)
			}
		case []any:
			for _, item := range v {
				if fields, ok := item.(map[string]any); ok {
					if k == "cookies" {
						fields["value"] = redactedValue
					}
					redactMap(fields)
				}
			}
		case string:
			if k == "database" && strings.Contains(v, "=") {
				values[key] = redactedValue
			} else if strings.Contains(v, "://") {
				values[key] = redactURL(v)
			}
		}
	}
}

func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return redactedValue
	}
	if u.User != nil {
		u.User = url.User(redactedValue)
	}
	query := u.Query()
	for key := range query {
		query.Set(key, redactedValue)
	}
	u.RawQuery = query.Encode()
	if u.Fragment != "" {
		u.Fragment = redactedValue
	}
	return u.String()
}
