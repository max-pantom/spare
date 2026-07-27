package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	TypeString    = "string"
	TypeDirectory = "directory"
	TypeSize      = "size"
	TypeBoolean   = "boolean"
	TypeInteger   = "integer"
)

type Field struct {
	Type        string `json:"type" yaml:"type"`
	Label       string `json:"label" yaml:"label"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required,omitempty" yaml:"required,omitempty"`
	Default     any    `json:"default,omitempty" yaml:"default,omitempty"`
	Minimum     int64  `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     int64  `json:"maximum,omitempty" yaml:"maximum,omitempty"`
}

func Resolve(fields map[string]Field, input map[string]any) (map[string]any, error) {
	result := make(map[string]any, len(fields))
	for id, field := range fields {
		value, ok := input[id]
		if !ok || value == nil || value == "" {
			value = field.Default
		}
		if value == nil || value == "" {
			if field.Required {
				return nil, fmt.Errorf("%s is required", field.Label)
			}
			continue
		}
		normalized, err := normalize(field, value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", field.Label, err)
		}
		result[id] = normalized
	}
	for id := range input {
		if _, ok := fields[id]; !ok {
			return nil, fmt.Errorf("unknown configuration field %q", id)
		}
	}
	return result, nil
}

func normalize(field Field, value any) (any, error) {
	switch field.Type {
	case TypeString, TypeDirectory:
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, errors.New("enter a value")
		}
		return strings.TrimSpace(text), nil
	case TypeSize:
		size, err := ParseSize(value)
		if err != nil {
			return nil, err
		}
		if field.Minimum > 0 && size < field.Minimum {
			return nil, fmt.Errorf("choose at least %d bytes", field.Minimum)
		}
		if field.Maximum > 0 && size > field.Maximum {
			return nil, fmt.Errorf("choose no more than %d bytes", field.Maximum)
		}
		return size, nil
	case TypeBoolean:
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case string:
			parsed, err := strconv.ParseBool(typed)
			if err != nil {
				return nil, errors.New("choose true or false")
			}
			return parsed, nil
		default:
			return nil, errors.New("choose true or false")
		}
	case TypeInteger:
		number, err := integer(value)
		if err != nil {
			return nil, err
		}
		if field.Minimum != 0 && number < field.Minimum {
			return nil, fmt.Errorf("choose %d or greater", field.Minimum)
		}
		if field.Maximum != 0 && number > field.Maximum {
			return nil, fmt.Errorf("choose %d or less", field.Maximum)
		}
		return number, nil
	default:
		return nil, fmt.Errorf("unsupported field type %q", field.Type)
	}
}

func ParseSize(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, errors.New("size is too large")
		}
		return int64(typed), nil
	case float64:
		if typed < 0 || typed != float64(int64(typed)) {
			return 0, errors.New("enter a whole number of bytes")
		}
		return int64(typed), nil
	case string:
		text := strings.ToUpper(strings.TrimSpace(typed))
		multiplier := int64(1)
		for _, suffix := range []struct {
			name       string
			multiplier int64
		}{
			{"TIB", 1 << 40}, {"TB", 1_000_000_000_000},
			{"GIB", 1 << 30}, {"GB", 1_000_000_000},
			{"MIB", 1 << 20}, {"MB", 1_000_000},
			{"KIB", 1 << 10}, {"KB", 1_000},
			{"B", 1},
		} {
			if strings.HasSuffix(text, suffix.name) {
				text = strings.TrimSpace(strings.TrimSuffix(text, suffix.name))
				multiplier = suffix.multiplier
				break
			}
		}
		number, err := strconv.ParseFloat(text, 64)
		if err != nil || number < 0 {
			return 0, errors.New("enter a size such as 500MB or 2GB")
		}
		result := number * float64(multiplier)
		if result > float64(^uint64(0)>>1) {
			return 0, errors.New("size is too large")
		}
		return int64(result), nil
	default:
		return 0, errors.New("enter a size such as 500MB or 2GB")
	}
}

func integer(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int64:
		return typed, nil
	case float64:
		if typed != float64(int64(typed)) {
			return 0, errors.New("enter a whole number")
		}
		return int64(typed), nil
	case string:
		number, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, errors.New("enter a whole number")
		}
		return number, nil
	default:
		return 0, errors.New("enter a whole number")
	}
}
