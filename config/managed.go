// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// loadManaged adds sparse web selections only where no authoritative source
// selects the setting. Reading configuration never rewrites either file.
func (inputs *configInputs) loadManaged(directory string) error {
	name := filepath.Join(directory, "config.web.yml")
	info, err := os.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read managed configuration %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("managed configuration %s must be a regular file", name)
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read managed configuration %s: %w", name, err)
	}
	if err := validateManagedYAML(raw); err != nil {
		return fmt.Errorf("managed configuration %s: %w", name, err)
	}
	web, err := fileInputs(raw, name)
	if err != nil {
		return fmt.Errorf("managed configuration %s: %w", name, err)
	}
	for path := range web.sources {
		if _, locked := inputs.sources[path]; !locked {
			if err := inputs.apply(Override{Path: path, Value: web.values.Get(path), Source: ValueSource{Kind: "web", Name: name}}); err != nil {
				return err
			}
		}
		source := inputs.sources[path]
		source.WebSelected = true
		inputs.sources[path] = source
	}
	return nil
}

func validateManagedYAML(raw []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(raw))
	var root yaml.Node
	if err := decoder.Decode(&root); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return errors.New("invalid YAML")
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("expected a single YAML mapping")
	}
	return validateManagedNode(root.Content[0], reflect.TypeFor[Config](), "")
}

// Validate the serialized shape before merging, including shadowed selections.
// Untyped extension options stay file managed until their owners supply metadata.
func validateManagedNode(node *yaml.Node, typ reflect.Type, path string) error {
	invalid := func() error { return fmt.Errorf("unsupported setting or value at %s", path) }
	switch path {
	case "app.directory", "server.database", "server.proxy_auth_header", "indexer.pdf_parser.command", "indexer.pdf_parser.args":
		return fmt.Errorf("%s is file managed", path)
	}
	if strings.HasPrefix(path, "server.oauth.") {
		switch path[strings.LastIndexByte(path, '.')+1:] {
		case "identity_claim", "admin_groups", "groups_claim":
			return fmt.Errorf("%s is file managed", path)
		}
	}
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if node.Kind == yaml.AliasNode || node.Tag == "!!null" {
		return invalid()
	}
	switch typ.Kind() {
	case reflect.Struct, reflect.Map:
		if node.Kind != yaml.MappingNode {
			return invalid()
		}
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Tag != "!!str" || seen[key.Value] {
				return invalid()
			}
			seen[key.Value] = true
			childPath := key.Value
			if path != "" {
				childPath = path + "." + key.Value
			}
			var childType reflect.Type
			if typ.Kind() == reflect.Map {
				childType = typ.Elem()
				// Named structural entries cannot contain path delimiters.
				if sourceSettingPath(path) == "" && (strings.Contains(key.Value, ".") || key.Value == "" || key.Value != strings.ToLower(key.Value)) {
					return invalid()
				}
			} else {
				for field := range typ.Fields() {
					if tag := field.Tag.Get("yaml"); tag != "" && tag != "-" && tag == key.Value {
						childType = field.Type
						break
					}
				}
			}
			if childType == nil {
				return fmt.Errorf("unknown setting at %s", childPath)
			}
			if err := validateManagedNode(node.Content[i+1], childType, childPath); err != nil {
				return err
			}
		}
	case reflect.Slice:
		if node.Kind != yaml.SequenceNode {
			return invalid()
		}
		for _, item := range node.Content {
			if err := validateManagedNode(item, typ.Elem(), path); err != nil {
				return err
			}
		}
	case reflect.String:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return invalid()
		}
	case reflect.Bool:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
			return invalid()
		}
	case reflect.Int, reflect.Int64:
		if node.Kind != yaml.ScalarNode || node.Tag != "!!int" {
			return invalid()
		}
	case reflect.Float64:
		if node.Kind != yaml.ScalarNode || (node.Tag != "!!float" && node.Tag != "!!int") {
			return invalid()
		}
	default:
		return invalid()
	}
	if node.Kind == yaml.ScalarNode {
		// Check representation limits before a weak decoder can coerce or wrap
		// numeric values, even when an authoritative selection shadows them.
		if err := node.Decode(reflect.New(typ).Interface()); err != nil {
			return invalid()
		}
	}
	return nil
}

// Main file maps historically inherit default members. Only collections
// selected by the web layer replace their defaults as a whole.
func (inputs *configInputs) clearWebDefaults(value reflect.Value, path string) {
	if value.Kind() == reflect.Map || value.Kind() == reflect.Slice {
		if inputs.sources[path].Kind == "web" {
			value.SetZero()
		}
		return
	}
	if value.Kind() != reflect.Struct {
		return
	}
	for field := range value.Type().Fields() {
		key := field.Tag.Get("mapstructure")
		if key == "" || key == "-" {
			continue
		}
		if path != "" {
			key = path + "." + key
		}
		inputs.clearWebDefaults(value.FieldByIndex(field.Index), key)
	}
}
