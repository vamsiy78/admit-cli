package schema

import (
	"admit/internal/contract"
	"admit/internal/invariant"
)

// ConfigType represents the type of a config value
type ConfigType string

const (
	TypeString ConfigType = "string"
	TypeEnum   ConfigType = "enum"
)

// ConfigKey represents a single configuration requirement
type ConfigKey struct {
	Path     string     // e.g., "db.url"
	Type     ConfigType // string or enum
	Required bool
	Values   []string // For enum type only
}

// Schema represents the full configuration schema
type Schema struct {
	Config       map[string]ConfigKey
	Invariants   []invariant.Invariant
	Environments map[string]contract.Contract // Environment contracts
}
