package config

import (
	"crypto/rand"
	"errors"
	"os"
)

// PrepareDataDirectory initializes storage files when a full reload switches
// data directories. It does not change the main configuration or its overrides.
func (c *Config) PrepareDataDirectory() error {
	if err := os.MkdirAll(c.App.Directory, 0o750); err != nil {
		return err
	}
	key, err := os.ReadFile(c.FullPath(secretKeyFilename))
	if errors.Is(err, os.ErrNotExist) {
		key = []byte(rand.Text() + rand.Text())
		f, err := os.OpenFile(c.FullPath(secretKeyFilename), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := f.Write(key)
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	c.secretKey = key
	if _, err := os.Stat(c.RulesPath()); errors.Is(err, os.ErrNotExist) {
		f, err := os.OpenFile(c.RulesPath(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := f.WriteString("{}\n")
		if err := errors.Join(writeErr, f.Close()); err != nil {
			return err
		}
	}
	return c.ReadRules()
}

// ReadRules requires an existing, valid rules file and never rewrites it.
func (c *Config) ReadRules() error {
	raw, err := os.ReadFile(c.RulesPath())
	if err != nil {
		return err
	}
	c.Rules = nil
	return c.parseRules(raw)
}

// Reload reads the original main configuration and rules without creating or
// modifying runtime files. Overrides preserve the running command's flags.
// The caller owns validation and replacement of runtime components.
func (c *Config) Reload(overrides ...func(*Config) error) (*Config, error) {
	next, err := loadSelected(c.fname, true)
	if err != nil {
		return nil, err
	}
	for _, override := range overrides {
		if err := override(next); err != nil {
			return nil, err
		}
	}
	if err := next.Validate(); err != nil {
		return nil, err
	}
	// Always read the running instance's rules. A candidate data directory
	// may not exist yet and must be initialized by the runtime owner.
	raw, err := os.ReadFile(c.RulesPath())
	if err != nil {
		return nil, errors.New("cannot read rules.json; active configuration retained")
	}
	if err := next.parseRules(raw); err != nil {
		return nil, errors.New("invalid rules.json; active configuration retained")
	}
	// These belong to the existing session store and terminal clients.
	next.secretKey = c.secretKey
	next.TUI = c.TUI
	next.tuiPath = c.tuiPath
	next.Hotkeys.TUI = c.Hotkeys.TUI
	return next, nil
}
