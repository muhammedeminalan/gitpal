package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Keys returns the dotted names of every setting, sorted.
func Keys() []string {
	keys := make([]string, 0, len(settingKeys))
	for key := range settingKeys {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// ErrUnknownKey is returned for a setting name gitpal does not have.
var ErrUnknownKey = errors.New("unknown setting")

// Get returns one setting of cfg as text: lists comma-separated, booleans as
// "true"/"false".
func Get(cfg Config, key string) (string, error) {
	field, ok := fieldByKey(reflect.ValueOf(cfg), key)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	switch field.Kind() {
	case reflect.Slice:
		parts := make([]string, field.Len())
		for i := range parts {
			parts[i] = field.Index(i).String()
		}
		return strings.Join(parts, ","), nil
	case reflect.Bool:
		return strconv.FormatBool(field.Bool()), nil
	case reflect.Int:
		return strconv.FormatInt(field.Int(), 10), nil
	default:
		return field.String(), nil
	}
}

// fieldByKey finds the struct field a dotted key names.
func fieldByKey(v reflect.Value, key string) (reflect.Value, bool) {
	for part := range strings.SplitSeq(key, ".") {
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, false
		}
		found := false
		for i := range v.NumField() {
			if v.Type().Field(i).Tag.Get("mapstructure") == part {
				v, found = v.Field(i), true
				break
			}
		}
		if !found {
			return reflect.Value{}, false
		}
	}
	return v, v.Kind() != reflect.Struct
}

// SetGlobal changes one setting in the user's global file, creating the file
// if needed. The rest of the file, comments included, is kept.
//
// The value is checked the way Load checks the file: the file is only
// replaced when the result loads cleanly, so a typo can never leave gitpal
// unable to start.
func SetGlobal(path, key, value string) error {
	field, ok := fieldByKey(reflect.ValueOf(Default()), key)
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownKey, key)
	}
	node, err := valueNode(field.Kind(), value)
	if err != nil {
		return &ValidationError{Key: key, Value: value}
	}

	data, err := os.ReadFile(path) //nolint:gosec // G304: the user's own config file
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return &FileError{Path: path, Err: err}
	}
	var doc yaml.Node
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return &FileError{Path: path, Err: err}
		}
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return &FileError{Path: path, Err: errors.New("the file is not a mapping of settings")}
	}
	setPath(doc.Content[0], strings.Split(key, "."), node)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return &FileError{Path: path, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return &FileError{Path: path, Err: err}
	}
	tmp := path + ".new"
	if err := os.WriteFile(tmp, out, 0o600); err != nil {
		return &FileError{Path: path, Err: err}
	}
	if _, err := Load(LoadOptions{GlobalPath: tmp}); err != nil {
		_ = os.Remove(tmp)
		var fileErr *FileError
		if errors.As(err, &fileErr) {
			var invalid *ValidationError
			if errors.As(fileErr.Err, &invalid) {
				return invalid
			}
		}
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return &FileError{Path: path, Err: err}
	}
	return nil
}

// valueNode turns the text a user typed into a YAML value of the setting's
// type.
func valueNode(kind reflect.Kind, value string) (*yaml.Node, error) {
	value = strings.TrimSpace(value)
	switch kind {
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: strconv.FormatBool(b)}, nil
	case reflect.Int:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, err
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.Itoa(n)}, nil
	case reflect.Slice:
		list := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for item := range strings.SplitSeq(value, ",") {
			if item = strings.TrimSpace(item); item != "" {
				list.Content = append(list.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: item})
			}
		}
		return list, nil
	default:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}, nil
	}
}

// setPath sets mapping[path...] = value, creating nested mappings as needed.
func setPath(mapping *yaml.Node, path []string, value *yaml.Node) {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != path[0] {
			continue
		}
		if len(path) == 1 {
			mapping.Content[i+1] = value
			return
		}
		child := mapping.Content[i+1]
		if child.Kind != yaml.MappingNode {
			child = &yaml.Node{Kind: yaml.MappingNode}
			mapping.Content[i+1] = child
		}
		setPath(child, path[1:], value)
		return
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: path[0]}
	if len(path) == 1 {
		mapping.Content = append(mapping.Content, keyNode, value)
		return
	}
	child := &yaml.Node{Kind: yaml.MappingNode}
	mapping.Content = append(mapping.Content, keyNode, child)
	setPath(child, path[1:], value)
}
