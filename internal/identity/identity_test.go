package identity

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"admit/internal/artifact"
	"admit/internal/cli"
)

// sha256HashPattern matches a valid sha256: prefixed hex string
var sha256HashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// genConfigArtifact generates a random ConfigArtifact
func genConfigArtifact() gopter.Gen {
	return gen.MapOf(gen.Identifier(), gen.AlphaString()).Map(func(values map[string]string) artifact.ConfigArtifact {
		return artifact.ConfigArtifact{
			ConfigVersion: artifact.ComputeConfigVersion(values),
			Values:        values,
		}
	})
}

// genCommand generates a random Command (using echo as a real executable)
func genCommand() gopter.Gen {
	return gen.SliceOf(gen.AlphaString()).Map(func(args []string) cli.Command {
		return cli.Command{
			Target: "echo", // Use a real command that exists
			Args:   args,
		}
	})
}

// TestExecutionIdentityStructure_Property tests Property 7: Execution Identity Structure
// Feature: admit-v1-config-artifact, Property 7: Execution Identity Structure
// For any execution identity, it SHALL contain codeHash, configHash, and executionId fields,
// where executionId equals {codeHash}:{configHash}.
// **Validates: Requirements 4.2, 5.1, 5.2**
func TestExecutionIdentityStructure_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("identity has all required fields", prop.ForAll(
		func(art artifact.ConfigArtifact) bool {
			cmd := cli.Command{Target: "echo", Args: []string{"test"}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			return identity.CodeHash != "" &&
				identity.ConfigHash != "" &&
				identity.ExecutionID != ""
		},
		genConfigArtifact(),
	))

	properties.Property("executionId equals codeHash:configHash", prop.ForAll(
		func(art artifact.ConfigArtifact) bool {
			cmd := cli.Command{Target: "echo", Args: []string{"test"}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			expected := identity.CodeHash + ":" + identity.ConfigHash
			return identity.ExecutionID == expected
		},
		genConfigArtifact(),
	))

	properties.Property("identity can be serialized to valid JSON", prop.ForAll(
		func(art artifact.ConfigArtifact) bool {
			cmd := cli.Command{Target: "echo", Args: []string{"test"}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			jsonBytes, err := identity.ToJSON()
			if err != nil {
				return false
			}

			var parsed ExecutionIdentity
			return json.Unmarshal(jsonBytes, &parsed) == nil
		},
		genConfigArtifact(),
	))

	properties.Property("Short() returns executionId", prop.ForAll(
		func(art artifact.ConfigArtifact) bool {
			cmd := cli.Command{Target: "echo", Args: []string{"test"}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			return identity.Short() == identity.ExecutionID
		},
		genConfigArtifact(),
	))

	properties.TestingRun(t)
}


// TestCodeHashFromFileContent_Property tests Property 8: Code Hash from File Content
// Feature: admit-v1-config-artifact, Property 8: Code Hash from File Content
// For any executable file, the code hash SHALL be the SHA-256 hash of the file's content,
// prefixed with "sha256:".
// **Validates: Requirements 4.3**
func TestCodeHashFromFileContent_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("code hash has valid sha256 format", prop.ForAll(
		func(args []string) bool {
			cmd := cli.Command{Target: "echo", Args: args}
			hash, err := ComputeCodeHash(cmd)
			if err != nil {
				return false
			}

			return sha256HashPattern.MatchString(hash)
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.Property("same command produces same code hash", prop.ForAll(
		func(args []string) bool {
			cmd := cli.Command{Target: "echo", Args: args}
			hash1, err1 := ComputeCodeHash(cmd)
			hash2, err2 := ComputeCodeHash(cmd)

			if err1 != nil || err2 != nil {
				return false
			}

			return hash1 == hash2
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.Property("non-existent command falls back to string hash", prop.ForAll(
		func(cmdName string) bool {
			// Use a command name that definitely doesn't exist
			cmd := cli.Command{Target: "nonexistent_cmd_" + cmdName, Args: []string{}}
			hash, err := ComputeCodeHash(cmd)
			if err != nil {
				return false
			}

			// Should still produce a valid hash
			return sha256HashPattern.MatchString(hash)
		},
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}


// TestConfigHashConsistency_Property tests Property 9: Config Hash Consistency
// Feature: admit-v1-config-artifact, Property 9: Config Hash Consistency
// For any config artifact and execution identity generated from it, the identity's
// configHash SHALL equal the artifact's configVersion.
// **Validates: Requirements 4.4**
func TestConfigHashConsistency_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("identity configHash equals artifact configVersion", prop.ForAll(
		func(values map[string]string) bool {
			art := artifact.ConfigArtifact{
				ConfigVersion: artifact.ComputeConfigVersion(values),
				Values:        values,
			}

			cmd := cli.Command{Target: "echo", Args: []string{"test"}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			return identity.ConfigHash == art.ConfigVersion
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
	))

	properties.TestingRun(t)
}


// TestIdentityIdempotence_Property tests Property 10: Identity Idempotence
// Feature: admit-v1-config-artifact, Property 10: Identity Idempotence
// For any executable and config combination, computing the execution identity multiple
// times SHALL produce identical results.
// **Validates: Requirements 4.5**
func TestIdentityIdempotence_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("same inputs produce same identity", prop.ForAll(
		func(values map[string]string, args []string) bool {
			art := artifact.ConfigArtifact{
				ConfigVersion: artifact.ComputeConfigVersion(values),
				Values:        values,
			}

			cmd := cli.Command{Target: "echo", Args: args}

			identity1, err1 := ComputeIdentity(cmd, art)
			identity2, err2 := ComputeIdentity(cmd, art)

			if err1 != nil || err2 != nil {
				return false
			}

			return identity1.CodeHash == identity2.CodeHash &&
				identity1.ConfigHash == identity2.ConfigHash &&
				identity1.ExecutionID == identity2.ExecutionID
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
		gen.SliceOf(gen.AlphaString()),
	))

	properties.TestingRun(t)
}


// TestHashFormatCompliance_Property tests Property 11: Hash Format Compliance
// Feature: admit-v1-config-artifact, Property 11: Hash Format Compliance
// For any hash value (codeHash, configHash, configVersion), it SHALL be formatted
// as lowercase hexadecimal with "sha256:" prefix.
// **Validates: Requirements 5.4**
func TestHashFormatCompliance_Property(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("codeHash has sha256: prefix and lowercase hex", prop.ForAll(
		func(args []string) bool {
			cmd := cli.Command{Target: "echo", Args: args}
			hash, err := ComputeCodeHash(cmd)
			if err != nil {
				return false
			}

			return sha256HashPattern.MatchString(hash)
		},
		gen.SliceOf(gen.AlphaString()),
	))

	properties.Property("configHash has sha256: prefix and lowercase hex", prop.ForAll(
		func(values map[string]string) bool {
			art := artifact.ConfigArtifact{
				ConfigVersion: artifact.ComputeConfigVersion(values),
				Values:        values,
			}

			cmd := cli.Command{Target: "echo", Args: []string{}}
			identity, err := ComputeIdentity(cmd, art)
			if err != nil {
				return false
			}

			return sha256HashPattern.MatchString(identity.ConfigHash)
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
	))

	properties.Property("configVersion has sha256: prefix and lowercase hex", prop.ForAll(
		func(values map[string]string) bool {
			version := artifact.ComputeConfigVersion(values)
			return sha256HashPattern.MatchString(version)
		},
		gen.MapOf(gen.Identifier(), gen.AlphaString()),
	))

	properties.TestingRun(t)
}
