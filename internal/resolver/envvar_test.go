package resolver

import (
	"strings"
	"testing"
	"unicode"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Feature: admit-cli, Property 4: Path to Environment Variable Conversion
// Validates: Requirements 3.1
// For any config path string, PathToEnvVar SHALL convert it to uppercase
// with dots replaced by underscores (e.g., "a.b.c" → "A_B_C").
func TestPathToEnvVar_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Property: result is uppercase
	properties.Property("result is uppercase", prop.ForAll(
		func(path string) bool {
			result := PathToEnvVar(path)
			for _, r := range result {
				if unicode.IsLetter(r) && !unicode.IsUpper(r) {
					return false
				}
			}
			return true
		},
		gen.AlphaString(),
	))

	// Property: dots are replaced with underscores
	properties.Property("dots replaced with underscores", prop.ForAll(
		func(path string) bool {
			result := PathToEnvVar(path)
			return !strings.Contains(result, ".")
		},
		gen.AlphaString().Map(func(s string) string {
			// Add some dots to test the replacement
			if len(s) > 2 {
				return s[:len(s)/2] + "." + s[len(s)/2:]
			}
			return s
		}),
	))

	// Property: conversion is deterministic (same input -> same output)
	properties.Property("conversion is deterministic", prop.ForAll(
		func(path string) bool {
			return PathToEnvVar(path) == PathToEnvVar(path)
		},
		gen.AlphaString(),
	))

	// Property: empty string returns empty string
	properties.Property("empty string returns empty", prop.ForAll(
		func(_ int) bool {
			return PathToEnvVar("") == ""
		},
		gen.Int(),
	))

	// Property: result equals uppercase with dots replaced
	properties.Property("result equals uppercase with dots replaced", prop.ForAll(
		func(path string) bool {
			result := PathToEnvVar(path)
			expected := strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
			return result == expected
		},
		gen.AlphaString().Map(func(s string) string {
			// Generate paths with dots
			if len(s) > 3 {
				return s[:len(s)/3] + "." + s[len(s)/3:2*len(s)/3] + "." + s[2*len(s)/3:]
			}
			return s
		}),
	))

	properties.TestingRun(t)
}
