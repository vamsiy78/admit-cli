# Admit CLI

A system-level execution gate that validates runtime configuration before launching applications. If config is invalid, the process never starts.

## Why Admit?

**The Problem:** Runtime configuration errors are one of the most common causes of production incidents. Applications start with missing database URLs, invalid API keys, or wrong environment settings - then crash minutes or hours later when that code path is hit. By then, you're debugging in production.

**Traditional approaches fail:**
- **Environment variable checks in code** - Scattered, inconsistent, often forgotten. The app starts, then crashes later.
- **Config libraries** - Still require code changes, still let the app start before validation.
- **Container health checks** - Only catch issues after the container is running.
- **CI/CD validation** - Catches typos but not runtime environment mismatches.

**Admit's approach:** Validate configuration *before* the process even starts. If config is invalid, `execve` never happens. Your application code never runs with bad config.

```
Without Admit:                    With Admit:
┌─────────────┐                   ┌─────────────┐
│ App Starts  │                   │ Admit Runs  │
└──────┬──────┘                   └──────┬──────┘
       │                                 │
       ▼                                 ▼
┌─────────────┐                   ┌─────────────┐
│ Config Load │                   │  Validate   │
│  (maybe)    │                   │   Config    │
└──────┬──────┘                   └──────┬──────┘
       │                                 │
       ▼                          ┌──────┴──────┐
┌─────────────┐                   │             │
│   Run...    │              [INVALID]     [VALID]
│   Run...    │                   │             │
│   Run...    │                   ▼             ▼
└──────┬──────┘             ┌─────────┐   ┌─────────┐
       │                    │  Exit   │   │  execve │
       ▼                    │  Error  │   │  (app)  │
┌─────────────┐             └─────────┘   └─────────┘
│   CRASH!    │              App never     App runs
│ Missing DB  │              started       with valid
│   config    │                            config
└─────────────┘
```

## Key Benefits

- **Fail fast, fail early** - Bad config = no process. Period.
- **Zero code changes** - Works with any application, any language. Just wrap your command.
- **Declarative schemas** - Define config requirements in YAML, not scattered code.
- **Runtime invariants** - Enforce rules like "prod environment must use prod database".
- **Container-native** - Perfect for Docker ENTRYPOINT, Kubernetes probes, CI pipelines.
- **Complete error reporting** - All validation errors shown at once, not one at a time.
- **Audit trail** - Content-addressable config artifacts for compliance and debugging.

## Overview

Admit is a launcher primitive that owns the execve boundary. It reads a schema file declaring configuration requirements, resolves values from environment variables, validates them against constraints, and only executes the target command if all invariants are satisfied.

**Core Principle:** `app_started ⟹ config_valid`

## Use Cases

| Scenario | How Admit Helps |
|----------|-----------------|
| **Production deployments** | Prevent starts with wrong DB, missing secrets, invalid feature flags |
| **Container orchestration** | Health checks via `admit check`, ENTRYPOINT validation |
| **CI/CD pipelines** | Pre-flight config validation with GitHub Actions annotations |
| **Multi-environment apps** | Invariants ensure prod config in prod, staging in staging |
| **Compliance & auditing** | Immutable config artifacts with content-addressable hashes |
| **Debugging** | Execution identity ties code version + config version together |

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

## V2 Features: Runtime Invariants

Invariants are declarative rules that enforce cross-configuration constraints at execution time. They prevent catastrophic misconfigurations like connecting to production databases in non-production environments.

### Declaring Invariants

Add an `invariants` section to your `admit.yaml`:

```yaml
config:
  db.url:
    type: string
    required: true
  db.url.env:
    type: enum
    values: [dev, staging, prod]
    required: true
  payments.mode:
    type: enum
    values: [sandbox, live]
    required: true

invariants:
  - name: prod-db-guard
    rule: execution.env == "prod" => db.url.env == "prod"
  
  - name: payments-flag-guard
    rule: execution.env != "prod" => payments.mode != "live"
  
  - name: staging-isolation
    rule: execution.env == "staging" => db.url.env == "staging"
```

### Rule Expression Syntax

Invariant rules support:

- **Implication**: `A => B` or `A ⇒ B` (if A is true, then B must be true)
- **Equality**: `A == B` (values must match)
- **Inequality**: `A != B` (values must differ)
- **Config references**: Dot-notation paths like `db.url.env`
- **Execution environment**: `execution.env` (reads from `ADMIT_ENV`)
- **String literals**: Quoted strings like `"prod"`

### Execution Environment

Set `ADMIT_ENV` to specify the execution environment:

```bash
# Production environment - invariants will enforce prod constraints
ADMIT_ENV=prod DB_URL="..." DB_URL_ENV=prod admit run node server.js

# Development environment - more relaxed constraints
ADMIT_ENV=dev DB_URL="..." DB_URL_ENV=dev admit run node server.js
```

### Invariant Violations

When an invariant fails, admit blocks execution and reports the violation:

```bash
ADMIT_ENV=prod DB_URL="postgres://staging-db/app" DB_URL_ENV=staging admit run node server.js
# Output:
# INVARIANT VIOLATION: 'prod-db-guard'
#   Rule: execution.env == "prod" => db.url.env == "prod"
#   execution.env is "prod" but db.url.env is "staging" (expected "prod")
```

Multiple violations are all reported before exiting:

```bash
# Output:
# INVARIANT VIOLATION: 'prod-db-guard'
#   Rule: execution.env == "prod" => db.url.env == "prod"
#   ...
# INVARIANT VIOLATION: 'payments-flag-guard'
#   Rule: execution.env != "prod" => payments.mode != "live"
#   ...
```

### JSON Output

Get invariant results as JSON for programmatic processing:

```bash
admit run --invariants-json node server.js
```

Output:
```json
{
  "invariants": [
    {
      "name": "prod-db-guard",
      "rule": "execution.env == \"prod\" => db.url.env == \"prod\"",
      "passed": false,
      "leftValue": "prod",
      "rightValue": "staging",
      "message": "Invariant 'prod-db-guard' failed: ..."
    }
  ],
  "allPassed": false,
  "failedCount": 1
}
```

### Common Invariant Patterns

| Scenario | Invariant Example |
|----------|-------------------|
| Wrong DB in prod | `execution.env == "prod" => db.url.env == "prod"` |
| Accidental flag enable | `execution.env != "prod" => payments.mode != "live"` |
| Region misconfigs | `execution.env == "prod" => region == "us-east-1"` |
| Feature flag consistency | `feature.v2 == "enabled" => api.version == "v2"` |

### Backward Compatibility

Invariants are opt-in. Schemas without an `invariants` section work exactly as before:

```bash
# No invariants defined - just v0/v1 validation
admit run node server.js
```

## V3 Features: Container & CI Enforcement

V3 adds features for container health checks, CI pipeline integration, and flexible schema location.

### Check Subcommand

Validate configuration without executing any command:

```bash
# Basic check - validates config and exits
admit check

# Check with JSON output
admit check --json
```

Output:
```json
{
  "valid": true,
  "validationErrors": [],
  "invariantResults": [],
  "schemaPath": "/app/admit.yaml"
}
```

Use `admit check` for:
- Container health checks (Kubernetes liveness/readiness probes)
- Pre-flight validation in CI pipelines
- Configuration testing without side effects

### Schema Path Flexibility

Specify schema location via flag or environment variable:

```bash
# Via --schema flag (highest priority)
admit check --schema /etc/admit/schema.yaml
admit run --schema ./config/admit.yaml node server.js

# Via ADMIT_SCHEMA environment variable
ADMIT_SCHEMA=/etc/admit/schema.yaml admit check

# Default: admit.yaml in current directory
admit check
```

Priority order:
1. `--schema` flag
2. `ADMIT_SCHEMA` environment variable
3. `admit.yaml` in current directory

### Dry-Run Mode

Validate configuration and see what would execute without actually running:

```bash
# Basic dry-run
admit run --dry-run node server.js
# Output: Config valid, would execute: node server.js

# Dry-run with JSON output
admit run --dry-run --json node server.js
```

JSON output:
```json
{
  "valid": true,
  "command": "node",
  "args": ["server.js"],
  "schemaPath": "/app/admit.yaml"
}
```

### CI Mode

Output validation errors in GitHub Actions annotation format:

```bash
# Via --ci flag
admit run --ci node server.js

# Via ADMIT_CI environment variable
ADMIT_CI=true admit run node server.js

# Also detects standard CI=true environment variable
CI=true admit run node server.js
```

CI mode output for validation errors:
```
::error file=admit.yaml::db.url: required but DB_URL is not set
::error file=admit.yaml::payments.mode: required but PAYMENTS_MODE is not set

❌ Validation failed: 2 error(s)
```

CI mode output for invariant violations:
```
::error file=admit.yaml::INVARIANT VIOLATION: 'prod-db-guard' - condition 'execution.env == "prod"' is true but 'db.url.env == "prod"' is false

❌ Invariant check failed: 1 violation(s)
```

### Container Integration Examples

#### Docker ENTRYPOINT

```dockerfile
FROM node:20-alpine

COPY admit /usr/local/bin/admit
COPY admit.yaml /app/admit.yaml
COPY . /app

WORKDIR /app

# Use admit as entrypoint
ENTRYPOINT ["admit", "run"]
CMD ["node", "server.js"]
```

#### Kubernetes Health Checks

```yaml
apiVersion: v1
kind: Pod
spec:
  containers:
  - name: app
    image: myapp:latest
    livenessProbe:
      exec:
        command: ["admit", "check"]
      initialDelaySeconds: 5
      periodSeconds: 10
    readinessProbe:
      exec:
        command: ["admit", "check"]
      initialDelaySeconds: 5
      periodSeconds: 5
```

#### CI Pipeline Example (GitHub Actions)

```yaml
name: Deploy
on: [push]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Validate config
        run: |
          admit check --ci --schema ./config/admit.yaml
        env:
          DB_URL: ${{ secrets.DB_URL }}
          PAYMENTS_MODE: ${{ vars.PAYMENTS_MODE }}
          ADMIT_ENV: production
```

### Backward Compatibility

All v3 features are opt-in. Without new flags, admit behaves exactly as v2:

```bash
# Standard v2 behavior - validation, invariants, then exec
admit run node server.js
```

## V4 Features: Execution Identity

V4 adds deterministic execution identity - a fingerprint for every execution that answers "what exactly ran?" with a single referenceable ID.

### Why Execution Identity?

- **Factual debugging**: Every execution has a unique, reproducible fingerprint
- **Traceable rollbacks**: Know exactly what configuration ran in each deployment
- **Audit compliance**: Content-addressable proof of execution context

### Execution ID Computation

The execution ID is computed from three components:

```
execution_id = sha256(config_version + command_hash + environment_hash)
```

- **config_version**: Hash of validated configuration values
- **command_hash**: Hash of command and arguments
- **environment_hash**: Hash of schema-referenced environment variables only

### Basic Usage

```bash
# Output execution ID to stdout
admit run --execution-id node server.js
# Output: sha256:a1b2c3d4e5f6...

# Output full execution identity as JSON
admit run --execution-id-json node server.js
# Output:
# {
#   "executionId": "sha256:a1b2c3d4...",
#   "configVersion": "sha256:def456...",
#   "commandHash": "sha256:789ghi...",
#   "environmentHash": "sha256:jkl012...",
#   "command": "node",
#   "args": ["server.js"]
# }

# Write execution identity to file
admit run --execution-id-file /var/log/admit/execid.json node server.js

# Inject execution ID into environment variable
admit run --execution-id-env EXEC_ID node server.js
```

### Execution ID in Check and Dry-Run Modes

```bash
# Get execution ID without running (check mode uses placeholder command hash)
admit check --execution-id
# Output: sha256:...

# Get execution ID in dry-run mode
admit run --dry-run --execution-id node server.js
# Output: sha256:...

# JSON output includes execution ID
admit check --json
# Output includes "executionId": "sha256:..."

admit run --dry-run --json node server.js
# Output includes "executionId": "sha256:..."
```

### Combining with V1 Identity

V4 execution identity is independent of V1 identity (they serve different purposes):

- **V1 identity**: Hashes executable file content + config values
- **V4 execution ID**: Hashes config + command + arguments + relevant environment

Both can be used together:

```bash
admit run --identity --execution-id node server.js
# Outputs both V1 identity JSON and V4 execution ID
```

### Determinism Guarantees

The execution ID is fully deterministic:

- Same config values → same config_version
- Same command and arguments → same command_hash
- Same schema-referenced environment variables → same environment_hash
- No timestamps, process IDs, or random values included
- Only environment variables referenced by the schema affect the hash

### Backward Compatibility

All v4 features are opt-in. Without new flags, admit behaves exactly as v3:

```bash
# Standard v3 behavior - validation, invariants, then exec
admit run node server.js
```

## V5 Features: Execution Replay

V5 adds execution replay - the ability to store execution snapshots and replay them later. This eliminates "works on my machine" problems and enables reproducible debugging.

### Why Execution Replay?

- **Reproducible debugging**: Replay any past execution with exact same config and environment
- **Incident investigation**: Capture production execution context for later analysis
- **Audit compliance**: Store proof of what exactly ran with what configuration

### Snapshot Storage

Store execution snapshots with the `--snapshot` flag:

```bash
# Store snapshot during execution
admit run --snapshot node server.js

# Snapshot is stored at ~/.admit/snapshots/{execution_id}.json
# Or use ADMIT_SNAPSHOT_DIR to customize location
ADMIT_SNAPSHOT_DIR=/var/log/admit/snapshots admit run --snapshot node server.js
```

Snapshot content:
```json
{
  "executionId": "sha256:a1b2c3d4...",
  "configVersion": "sha256:def456...",
  "command": "node",
  "args": ["server.js"],
  "environment": {
    "DB_URL": "postgres://localhost/mydb",
    "PAYMENTS_MODE": "test"
  },
  "schemaPath": "/app/admit.yaml",
  "timestamp": "2025-12-31T10:30:00Z"
}
```

### Replay Subcommand

Replay a stored execution:

```bash
# Replay an execution by ID
admit replay sha256:a1b2c3d4...

# Preview what would be executed (dry-run)
admit replay --dry-run sha256:a1b2c3d4...
# Output:
# Would execute: node server.js
# With environment:
#   DB_URL=postgres://localhost/mydb
#   PAYMENTS_MODE=test

# Get snapshot as JSON
admit replay --json sha256:a1b2c3d4...
```

### Snapshot Management

List and manage stored snapshots:

```bash
# List all snapshots
admit snapshots
# Output:
# sha256:a1b2c3d4...  node  2025-12-31T10:30:00Z
# sha256:e5f6g7h8...  python  2025-12-30T15:45:00Z

# List as JSON
admit snapshots --json

# Delete a specific snapshot
admit snapshots --delete sha256:a1b2c3d4...

# Prune snapshots older than 30 days
admit snapshots --prune 30
```

### Snapshot Directory Configuration

```bash
# Default location
~/.admit/snapshots/

# Custom location via environment variable
ADMIT_SNAPSHOT_DIR=/var/log/admit/snapshots admit run --snapshot node server.js

# Snapshots are stored as {execution_id}.json
# The ':' in execution IDs is replaced with '_' for filesystem compatibility
```

### Snapshot Verification

When replaying, admit verifies snapshot integrity:

```bash
admit replay sha256:a1b2c3d4...
# Warning: snapshot may be corrupted (execution ID mismatch)
# Warning: schema file no longer exists
```

Verification checks:
- Execution ID matches computed ID from snapshot contents
- Schema file still exists at original path

### Combining with Other Features

```bash
# Store snapshot and output execution ID
admit run --snapshot --execution-id node server.js

# Store snapshot with all v4 features
admit run --snapshot --execution-id-json --artifact-file /tmp/config.json node server.js
```

### Backward Compatibility

All v5 features are opt-in. Without new flags, admit behaves exactly as v4:

```bash
# Standard v4 behavior - no snapshots stored
admit run node server.js
```

## V6 Features: Drift Detection

V6 adds passive drift detection - compare current configuration against a known-good baseline and report differences as warnings. Drift detection is purely informational and never blocks execution.

### Why Drift Detection?

- **Configuration awareness**: Know when config has changed from a known-good state
- **Gradual rollouts**: Detect unintended config changes during deployments
- **Debugging aid**: Quickly identify what changed between working and broken states
- **Non-blocking**: Warnings only - execution always proceeds

### Baseline Storage

Store a baseline during a successful execution:

```bash
# Store baseline with default name
admit run --baseline default node server.js

# Store baseline with custom name
admit run --baseline production-2025-01 node server.js

# Baselines are stored at ~/.admit/baselines/{name}.json
# Or use ADMIT_BASELINE_DIR to customize location
ADMIT_BASELINE_DIR=/var/log/admit/baselines admit run --baseline prod node server.js
```

Baseline content:
```json
{
  "name": "production-2025-01",
  "executionId": "sha256:a1b2c3d4...",
  "configHash": "sha256:def456...",
  "configValues": {
    "db.url": "postgres://prod-db/app",
    "payments.mode": "live"
  },
  "command": "node server.js",
  "timestamp": "2025-01-15T10:30:00Z"
}
```

### Drift Detection

Compare current configuration against a stored baseline:

```bash
# Detect drift against default baseline
admit run --detect-drift default node server.js

# Detect drift against named baseline
admit run --detect-drift production-2025-01 node server.js

# If baseline doesn't exist, execution continues silently (no error)
```

When drift is detected, warnings are printed to stderr:

```
⚠ Configuration drift detected from baseline 'production-2025-01'
  Baseline: sha256:def456... (2025-01-15T10:30:00Z)
  Current:  sha256:789abc...

  Changed keys:
    db.url: postgres://prod-db/app → postgres://staging-db/app
  
  Added keys:
    feature.new: enabled
  
  Removed keys:
    legacy.flag
```

### JSON Drift Output

Get drift report as JSON for programmatic processing:

```bash
admit run --detect-drift default --drift-json node server.js
```

Output:
```json
{
  "hasDrift": true,
  "baselineName": "default",
  "baselineHash": "sha256:def456...",
  "currentHash": "sha256:789abc...",
  "baselineTime": "2025-01-15T10:30:00Z",
  "changes": [
    {
      "key": "db.url",
      "type": "changed",
      "baselineValue": "postgres://prod-db/app",
      "currentValue": "postgres://staging-db/app"
    },
    {
      "key": "feature.new",
      "type": "added",
      "currentValue": "enabled"
    },
    {
      "key": "legacy.flag",
      "type": "removed",
      "baselineValue": "true"
    }
  ]
}
```

### CI Mode Drift Warnings

In CI mode, drift warnings use GitHub Actions annotation format:

```bash
ADMIT_CI=true admit run --detect-drift default node server.js
```

Output:
```
::warning file=admit.yaml::Configuration drift detected: db.url changed from 'postgres://prod-db/app' to 'postgres://staging-db/app'
::warning file=admit.yaml::Configuration drift detected: feature.new added with value 'enabled'
::warning file=admit.yaml::Configuration drift detected: legacy.flag removed (was 'true')
```

### Baseline Management

Manage stored baselines with the `baseline` subcommand:

```bash
# List all baselines
admit baseline list
# Output:
# default           sha256:def456...  2025-01-15T10:30:00Z
# production-2025-01  sha256:789abc...  2025-01-20T14:00:00Z

# Show baseline details
admit baseline show production-2025-01
# Output:
# Name: production-2025-01
# Execution ID: sha256:a1b2c3d4...
# Config Hash: sha256:789abc...
# Command: node server.js
# Timestamp: 2025-01-20T14:00:00Z
# Config Values:
#   db.url: postgres://prod-db/app
#   payments.mode: live

# Delete a baseline
admit baseline delete production-2025-01
```

### Baseline Directory Configuration

```bash
# Default location
~/.admit/baselines/

# Custom location via environment variable
ADMIT_BASELINE_DIR=/var/log/admit/baselines admit run --baseline prod node server.js

# Baselines are stored as {name}.json
```

### Passive Drift Philosophy

Drift detection is intentionally passive:

- **Never blocks execution**: Even with drift, the command runs
- **Warnings only**: Drift is reported to stderr as informational output
- **Silent on missing baseline**: If baseline doesn't exist, no error - just no comparison
- **No exit code impact**: Drift doesn't change the exit code

This design supports gradual adoption and prevents drift detection from becoming a deployment blocker.

### Combining with Other Features

```bash
# Store baseline and snapshot together
admit run --baseline prod --snapshot node server.js

# Detect drift with execution ID output
admit run --detect-drift prod --execution-id node server.js

# Full observability: baseline, drift detection, snapshot, and identity
admit run --baseline prod --detect-drift prod --snapshot --execution-id-json node server.js
```

### Backward Compatibility

All v6 features are opt-in. Without new flags, admit behaves exactly as v5:

```bash
# Standard v5 behavior - no baseline storage or drift detection
admit run node server.js
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Validation passed, command executed successfully |
| 1 | Validation failed |
| 2 | Invariant violation (v2+) |
| 3 | Schema error (file not found, parse error) (v3+) |
| 4 | Snapshot/baseline not found (v5+/v6+) |
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
│   ├── baseline/
│   │   ├── types.go             # V6 baseline data structures
│   │   ├── store.go             # Baseline storage operations
│   │   └── store_test.go        # Store property tests
│   ├── cli/
│   │   ├── parser.go            # CLI argument parsing
│   │   └── parser_test.go       # Argument preservation property tests
│   ├── drift/
│   │   ├── detector.go          # V6 drift detection logic
│   │   ├── detector_test.go     # Detector property tests
│   │   ├── reporter.go          # Drift report formatting
│   │   └── reporter_test.go     # Reporter property tests
│   ├── execid/
│   │   ├── execid.go            # V4 execution identity generation
│   │   └── execid_test.go       # Execution identity property tests
│   ├── identity/
│   │   ├── identity.go          # V1 execution identity generation
│   │   └── identity_test.go     # Identity property tests
│   ├── injector/
│   │   ├── injector.go          # Runtime config injection
│   │   └── injector_test.go     # Injection property tests
│   ├── invariant/
│   │   ├── types.go             # Invariant AST types
│   │   ├── parser.go            # Rule expression parser
│   │   ├── parser_test.go       # Parser property tests
│   │   ├── evaluator.go         # Invariant evaluation logic
│   │   ├── evaluator_test.go    # Evaluator property tests
│   │   ├── reporter.go          # Violation message formatting
│   │   └── reporter_test.go     # Reporter property tests
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
│   ├── snapshot/
│   │   ├── types.go             # V5 snapshot data structures
│   │   ├── store.go             # Snapshot storage operations
│   │   ├── store_test.go        # Store property tests
│   │   ├── verify.go            # Snapshot integrity verification
│   │   └── verify_test.go       # Verification tests
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

The implementation is validated by 48 property-based tests using [gopter](https://github.com/leanovate/gopter):

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

### V2 Properties (Runtime Invariants)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Schema Parsing with Invariants | Req 1.1, 1.4, 1.5 |
| 2 | Invariant Field Validation | Req 1.2, 1.3 |
| 3 | Rule Expression Round-Trip | Req 2.1-2.5, 2.8, 2.9 |
| 4 | Undefined Key Detection | Req 2.7 |
| 5 | Implication Evaluation Semantics | Req 3.2 |
| 6 | Equality/Inequality Evaluation | Req 3.3, 3.4 |
| 7 | Invariant Execution Gate | Req 4.1, 4.2, 4.7 |
| 8 | All Violations Reported | Req 4.6 |
| 9 | Violation Output Completeness | Req 4.3-4.5, 5.1-5.3 |
| 10 | JSON Output Format | Req 5.5 |
| 11 | Backward Compatibility - No Invariants | Req 6.1, 6.4 |

### V3 Properties (Container & CI Enforcement)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Check Validation Equivalence | Req 3.1-3.4 |
| 2 | Schema Path Resolution | Req 2.4, 2.5, 5.1-5.4 |
| 3 | Missing Schema Error | Req 5.5 |
| 4 | Dry Run Non-Execution | Req 6.1-6.4 |
| 5 | CI Annotation Format Compliance | Req 4.2, 4.3, 4.5 |
| 6 | Exit Code Consistency | Req 3.2-3.4, 7.4 |
| 7 | Backward Compatibility | Req 7.1-7.3 |
| 8 | Check JSON Output Structure | Req 3.5 |

### V4 Properties (Execution Identity)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Execution ID Determinism | Req 1.4, 2.4, 3.5, 7.1, 7.2, 7.4 |
| 2 | Hash Format Compliance | Req 1.3, 2.3, 3.4 |
| 3 | Execution ID Composition | Req 1.1, 1.2 |
| 4 | Command Hash Sensitivity | Req 2.1, 2.2 |
| 5 | Environment Hash Filtering | Req 3.1, 3.2, 7.3 |
| 6 | Environment Hash Order Independence | Req 3.3 |
| 7 | Execution Identity JSON Structure | Req 4.2, 4.3, 4.4 |
| 8 | Execution ID Injection | Req 4.5 |
| 9 | JSON Output Includes Execution ID | Req 5.3, 5.4 |
| 10 | Backward Compatibility | Req 6.1, 6.2, 6.3 |

### V5 Properties (Execution Replay)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Snapshot Storage | Req 1.1, 1.4, 1.5 |
| 2 | Snapshot Directory Configuration | Req 1.2, 1.3 |
| 3 | Snapshot JSON Structure | Req 2.1-2.8 |
| 4 | Replay Execution | Req 3.1, 3.2, 3.3 |
| 5 | Replay Missing Snapshot Error | Req 3.4 |
| 6 | Replay Dry Run | Req 3.5, 3.6 |
| 7 | Snapshot Listing | Req 4.1, 4.2, 4.3 |
| 8 | Snapshot Cleanup | Req 5.1, 5.2 |
| 9 | Snapshot Integrity Verification | Req 6.1, 6.2, 6.3, 6.4 |
| 10 | Backward Compatibility | Req 7.1, 7.2, 7.3 |

### V6 Properties (Drift Detection)

| # | Property | Validates |
|---|----------|-----------|
| 1 | Baseline Round-Trip | Req 1.1, 1.2 |
| 2 | Baseline Directory Configuration | Req 1.3, 1.4 |
| 3 | Multiple Named Baselines | Req 1.5 |
| 4 | No Drift When Hashes Match | Req 2.3 |
| 5 | Drift Report Contains Key Differences | Req 2.4, 3.4, 3.5, 3.6, 3.7 |
| 6 | Drift Never Blocks Execution | Req 2.5 |
| 7 | Drift Report Formatting | Req 3.1, 3.2, 3.3 |
| 8 | Baseline List and Delete | Req 4.1, 4.3 |
| 9 | Backward Compatibility | Req 5.1, 5.3 |

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
