# Admit CLI

A system-level execution gate that validates runtime configuration before launching applications. If config is invalid, the process never starts.

## Overview

Admit is a launcher primitive that owns the execve boundary. It reads a schema file declaring configuration requirements, resolves values from environment variables, validates them against constraints, and only executes the target command if all invariants are satisfied.

**Core Principle:** `app_started ⟹ config_valid`

## Installation

```bash
# Build from source
go build -o admit ./cmd/admit

# Or use make
make build
```

## Quick Start

1. Create an `admit.yaml` schema in your project:

```yaml
config:
  db.url:
    type: string
    required: true

  payments.mode:
    type: enum
    values: [test, live]
    required: true

  log.level:
    type: enum
    values: [debug, info, warn, error]
    required: false
```

2. Run your application through admit:

```bash
# With valid config - command executes silently
DB_URL="postgres://localhost/mydb" PAYMENTS_MODE="test" admit run node index.js

# With missing required config - blocked with clear errors
admit run node index.js
# Output:
# db.url: required but DB_URL is not set
# payments.mode: required but PAYMENTS_MODE is not set

# With invalid enum value - blocked with allowed values listed
DB_URL="postgres://localhost/mydb" PAYMENTS_MODE="invalid" admit run node index.js
# Output:
# payments.mode: 'invalid' is not valid, must be one of: test, live
```

## Schema Format

The `admit.yaml` file declares configuration requirements:

```yaml
config:
  <config.path>:
    type: string | enum
    required: true | false
    values: [value1, value2]  # Required for enum type
```

### Config Path to Environment Variable

Config paths use dot-notation and are converted to uppercase underscore-separated environment variable names:

| Config Path | Environment Variable |
|-------------|---------------------|
| `db.url` | `DB_URL` |
| `payments.mode` | `PAYMENTS_MODE` |
| `app.server.port` | `APP_SERVER_PORT` |

### Supported Types

- **string**: Accepts any non-empty string value
- **enum**: Accepts only values from the declared `values` list

## Usage

```bash
admit run [flags] <command> [args...]
```

### Basic Examples

```bash
# Run a Node.js application
admit run node server.js

# Run npm scripts
admit run npm start

# Run with arguments
admit run python manage.py runserver 0.0.0.0:8000

# Run in Docker
docker run -e DB_URL=... -e PAYMENTS_MODE=test myimage admit run ./myapp
```

## V1 Features: Config Artifacts, Identity & Injection

### Config Artifact

After validation, admit can produce an immutable, content-addressable config artifact:

```bash
# Write artifact to file
admit run --artifact-file /tmp/config.json node server.js

# Print artifact to stdout
admit run --artifact-stdout node server.js

# Log configVersion to stderr
admit run --artifact-log node server.js
```

Artifact format:
```json
{
  "configVersion": "sha256:a1b2c3d4e5f6...",
  "values": {
    "db.url": "postgres://localhost/mydb",
    "payments.mode": "test"
  }
}
```

The `configVersion` is a SHA-256 hash of the canonical config content, providing:
- **Immutability**: Same config always produces same hash
- **Identifiability**: Different configs produce different hashes
- **Auditability**: Log and track config versions

### Runtime Injection

Pass config to applications without relying on environment variables:

```bash
# Inject config as a file
admit run --inject-file /tmp/app-config.json node server.js

# Inject config as an environment variable
admit run --inject-env ADMIT_CONFIG node server.js
```

Your application can then read the injected config:
```javascript
// Node.js example
const config = JSON.parse(process.env.ADMIT_CONFIG);
console.log(config.values['db.url']);
```

### Execution Identity

Every execution can have a unique identity based on code and config:

```bash
# Output full identity JSON
admit run --identity node server.js
# Output:
# {
#   "codeHash": "sha256:1234abcd...",
#   "configHash": "sha256:a1b2c3d4...",
#   "executionId": "sha256:1234abcd...:sha256:a1b2c3d4..."
# }

# Output only the short executionId
admit run --identity-short node server.js
# Output: sha256:1234abcd...:sha256:a1b2c3d4...

# Write identity to file
admit run --identity-file /tmp/identity.json node server.js
```

The execution identity enables:
- **Deterministic debugging**: Same code + config = same identity
- **Reproducibility**: Track exactly what ran with what config
- **Audit trails**: Log execution identities for compliance

### Combining Flags

All v1 flags can be combined:

```bash
admit run \
  --artifact-file /var/log/admit/config.json \
  --artifact-log \
  --inject-env APP_CONFIG \
  --identity-file /var/log/admit/identity.json \
  node server.js
```

### Backward Compatibility

All v1 features are opt-in. Without any new flags, admit behaves exactly as v0:

```bash
# No artifact output, no injection, no identity - just validation and exec
admit run node server.js
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Validation passed, command executed successfully |
| 1 | Validation failed or schema error |
| 126 | Command found but permission denied |
| 127 | Command not found |
| N | Exit code from the executed command |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    admit run <cmd>                       │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                   Schema Loader                          │
│  - Read admit.yaml from current directory               │
│  - Parse YAML into schema structure                     │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                     Resolver                             │
│  - Convert config paths to env var names                │
│  - Look up values from environment                      │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│                    Validator                             │
│  - Check required fields                                │
│  - Validate enum values                                 │
│  - Collect all errors                                   │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌───────────┴───────────┐
              │                       │
         [INVALID]               [VALID]
              │                       │
              ▼                       ▼
┌─────────────────────┐   ┌─────────────────────┐
│   Print errors      │   │   execve(cmd)       │
│   Exit non-zero     │   │   (replace process) │
└─────────────────────┘   └─────────────────────┘
```

## Project Structure

```
admit/
├── cmd/
│   └── admit/
│       ├── main.go              # Entry point and orchestration
│       ├── main_test.go         # Property tests for silent success
│       └── integration_test.go  # Integration tests
├── internal/
│   ├── artifact/
│   │   ├── artifact.go          # Config artifact generation
│   │   ├── artifact_test.go     # Artifact property tests
│   │   └── writer.go            # Artifact file writing
│   ├── cli/
│   │   ├── parser.go            # CLI argument parsing
│   │   └── parser_test.go       # Argument preservation property tests
│   ├── identity/
│   │   ├── identity.go          # Execution identity generation
│   │   └── identity_test.go     # Identity property tests
│   ├── injector/
│   │   ├── injector.go          # Runtime config injection
│   │   └── injector_test.go     # Injection property tests
│   ├── launcher/
│   │   └── exec.go              # execve wrapper
│   ├── resolver/
│   │   ├── envvar.go            # Path-to-env conversion
│   │   ├── envvar_test.go       # Conversion property tests
│   │   ├── resolver.go          # Environment variable resolution
│   │   └── resolver_test.go     # Resolution tests
│   ├── schema/
│   │   ├── types.go             # Schema data structures
│   │   ├── parser.go            # YAML parsing and serialization
│   │   └── parser_test.go       # Round-trip and invalid YAML tests
│   └── validator/
│       ├── validator.go         # Validation logic
│       ├── validator_test.go    # Validation property tests
│       ├── errors.go            # Error message formatting
│       └── errors_test.go       # Error format property tests
├── admit.yaml                   # Example schema
├── Makefile                     # Build commands
├── go.mod                       # Go module definition
└── go.sum                       # Dependency checksums
```


## Implementation Details

### CLI Module (`internal/cli/`)

Parses command-line arguments and extracts the target command:

```go
type Command struct {
    Target string   // The command to execute (e.g., "node")
    Args   []string // Arguments to pass (e.g., ["index.js"])
}

func ParseArgs(args []string) (Command, error)
```

- Expects `admit run <command> [args...]` format
- Returns error if `run` subcommand missing or no command provided
- Preserves all arguments in order for the target command

### Schema Module (`internal/schema/`)

Parses and represents configuration schemas:

```go
type ConfigType string

const (
    TypeString ConfigType = "string"
    TypeEnum   ConfigType = "enum"
)

type ConfigKey struct {
    Path     string     // e.g., "db.url"
    Type     ConfigType // string or enum
    Required bool
    Values   []string   // For enum type only
}

type Schema struct {
    Config map[string]ConfigKey
}

func LoadSchema(dir string) (Schema, error)
func ParseSchema(content []byte) (Schema, error)
func (s Schema) ToYAML() ([]byte, error)
```

- Loads `admit.yaml` from specified directory
- Validates type is `string` or `enum`
- Validates enum types have `values` defined
- Supports round-trip serialization (parse → serialize → parse)

### Resolver Module (`internal/resolver/`)

Resolves configuration values from environment variables:

```go
type ResolvedValue struct {
    Key     string // The config key path (e.g., "db.url")
    EnvVar  string // The environment variable name (e.g., "DB_URL")
    Value   string // The resolved value (empty if not set)
    Present bool   // Whether the env var was set
}

func PathToEnvVar(path string) string
func Resolve(schema Schema, environ []string) []ResolvedValue
```

- Converts dot-notation paths to uppercase underscore-separated names
- Handles environ slice format (`KEY=VALUE`)
- Handles values containing `=` characters
- Distinguishes between missing and empty values

### Validator Module (`internal/validator/`)

Validates resolved values against schema constraints:

```go
type ValidationError struct {
    Key     string   // The config key path
    EnvVar  string   // The environment variable name
    Message string   // Human-readable error message
    Value   string   // The invalid value (if present)
    Allowed []string // For enum errors, the allowed values
}

type ValidationResult struct {
    Valid  bool
    Errors []ValidationError
}

func Validate(schema Schema, resolved []ResolvedValue) ValidationResult
func FormatError(err ValidationError) string
```

- Checks required fields are present
- Validates enum values against allowed list
- Collects ALL errors (not just first)
- Formats errors with key, env var name, and context

### Launcher Module (`internal/launcher/`)

Executes the target command via execve:

```go
func Exec(cmd Command, environ []string) error
func IsNotFound(err error) bool
func IsPermissionDenied(err error) bool
```

- Uses `syscall.Exec` to replace the current process
- Passes through all environment variables
- Handles command not found (exit 127) and permission denied (exit 126)

### Main Orchestration (`cmd/admit/main.go`)

Coordinates the full execution flow:

1. Parse CLI arguments
2. Load schema from current directory
3. Resolve config from environment
4. Validate config against schema
5. If invalid: print all errors to stderr, exit 1
6. If valid: execve target command (silent success)

## Correctness Properties

The implementation is validated by 27 property-based tests using [gopter](https://github.com/leanovate/gopter):

### Core Properties (v0)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Argument Parsing Preservation | Req 1.2 |
| 2 | Invalid YAML Produces Parse Error | Req 2.3 |
| 3 | Schema Round-Trip | Req 2.4 |
| 4 | Path to Environment Variable Conversion | Req 3.1 |
| 5 | Required Field Validation | Req 4.1 |
| 6 | String Type Acceptance | Req 4.2 |
| 7 | Enum Validation Correctness | Req 4.3, 4.4 |
| 8 | Error Collection Completeness | Req 4.5 |
| 9 | Execution Gate Invariant | Req 5.1, 5.3 |
| 10 | Environment Passthrough | Req 5.4 |
| 11 | Exit Code Propagation | Req 5.5 |
| 12 | Missing Required Error Message | Req 6.1 |
| 13 | Invalid Enum Error Message | Req 6.2 |
| 14 | Silent Success | Req 6.4 |

### V1 Properties (Config Artifact, Identity, Injection)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Artifact Structure Validity | Req 1.1, 1.2, 1.3 |
| 2 | Canonical JSON Determinism | Req 1.4 |
| 3 | Config Hash Idempotence | Req 1.5 |
| 4 | Config Hash Uniqueness | Req 1.6 |
| 5 | Injected Environment Contains Artifact | Req 3.2, 3.3 |
| 6 | Environment Preservation with Injection | Req 3.4 |
| 7 | Execution Identity Structure | Req 4.2, 5.1, 5.2 |
| 8 | Code Hash from File Content | Req 4.3 |
| 9 | Config Hash Consistency | Req 4.4 |
| 10 | Identity Idempotence | Req 4.5 |
| 11 | Hash Format Compliance | Req 5.4 |
| 12 | Backward Compatibility | Req 6.1 |
| 13 | Validation Before Artifacts | Req 6.2, 6.3 |

Each property test runs 100 iterations with randomly generated inputs.

## Development

```bash
# Build
make build

# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Clean build artifacts
make clean

# Build and test
make check
```

## Dependencies

- Go 1.21+
- [gopkg.in/yaml.v3](https://github.com/go-yaml/yaml) - YAML parsing
- [github.com/leanovate/gopter](https://github.com/leanovate/gopter) - Property-based testing

## Design Principles

- **Zero friction**: No code changes required in target applications
- **Single binary**: No runtime dependencies
- **Fast startup**: Validation adds minimal latency
- **Clear boundaries**: Admit owns execve, nothing else
- **Fail fast**: Invalid config blocks execution immediately
- **Complete errors**: All validation errors reported, not just first

## License

MIT
