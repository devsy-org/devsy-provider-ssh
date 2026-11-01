# E2E tests

These tests run the SSH provider against real login shells and a local sshd.
Run Go commands from `e2e/` unless a step says otherwise.

## Layout

### `e2e/qualification`

Plain Go package named `qualification` with no Ginkgo or Gomega imports.
It is the single source for the report schema and the shell matrix.

- `SchemaVersion`, `PrivilegeUser`, and the lifecycle scenario names.
- `Transports`: the shell and transport matrix. `Shells()` returns the unique shell names.
- `Pair`, `Cell`, `Scenario`, `Privilege`, and `Report`: the JSON report types.
- `Load` reads a report file. `Report.Validate` checks it.

### `e2e/tests/integration`

Ginkgo specs in package `integration`.

- `labels.go`: label names used by the spec filters.
- `environment.go`: provider and core setup, SSH keys, the login account key, and `providerCommandEnv`.
- `compat_lifecycle.go`: the compatibility lifecycle suite. It uses the default core download.
- `cell.go`: one shell and transport cell. It holds setup, cleanup, and the qualification preparation.
- `lifecycle.go`: the full workspace lifecycle spec for each cell and its scenario checks.
- `provider_command.go`: the provider command stream and exit status specs.
- `privileged_install.go`: the `devsy-sudo` scenario that installs the agent with sudo.
- `pair.go`: the qualification environment gate, the explicit core and provider pair, and the report writer.

### `hack/ci`

CI commands. Run them from the repository root with `go run ./hack/ci <command>`.

- `provision-accounts`: create the `devsy-bash`, `devsy-zsh`, and `devsy-fish` accounts.
- `provision-qualification-accounts`: create the qualification accounts and the sudo fixture.
- `cleanup-qualification-accounts`: remove the qualification accounts and the sudo fixture.
- `qualify-core`: resolve and verify the candidate core.
- `verify-provider`: download the released provider and check its digest.
- `missing-candidate-guard`: fail when the explicit candidate is missing.
- `verify-report`: validate the qualification report.

## Run the compatibility suite

The default lifecycle suite downloads core v1.24.0 unless `DEVSY_E2E_BINARY` is set.
That run checks backward compatibility only. It is not release qualification.

```
go test -v -ginkgo.v -timeout 3600s --ginkgo.label-filter=integration
```

## Run the provider command matrix locally

Use a disposable Linux host or VM. The account command creates real login accounts.

1. Install the shells and sshd. On Ubuntu:
   ```
   sudo apt-get update && sudo apt-get install -y bash zsh fish openssh-server
   ```
2. Create the accounts from the repository root:
   ```
   go run ./hack/ci provision-accounts
   ```
3. Start sshd:
   ```
   sudo service ssh start
   ```
4. Run the matrix from `e2e/`:
   ```
   DEVSY_E2E_SHELL_MATRIX=true go test -v -ginkgo.v -timeout 3600s \
     --ginkgo.label-filter=provider-command
   ```

## Release qualification

Qualification checks one explicit core and provider pair. It does not pick a default core.

Start it by dispatching the `Tests` workflow with `core_tag` (for example `v1.25.0-beta.8`)
and `provider_tag` (default `v1.1.47`). The qualification job only runs when `core_tag` is set.
From a terminal:

```
gh workflow run tests.yaml -f core_tag=v1.25.0-beta.8 -f provider_tag=v1.1.47
```

The workflow runs two independent jobs. The `cli` job qualifies the standalone Linux amd64 CLI.
The `desktop` job qualifies the CLI extracted from the Desktop candidate's `Devsy_linux_amd64.deb`.
Each job verifies the published asset digest, runs that binary's `--version`, records its SHA256,
and checks that core `e22be00` and `9c80b29` and provider `d31fed1` are ancestors of the selected
release commits. The provider binary and checksums come from the selected provider release.

Each job uploads `release-qualification-<core_tag>-cli` or `release-qualification-<core_tag>-desktop`.
The upload holds `pair-verification.txt`, `qualification-report.json`, and `missing-binary-guard.log`.

### Why Ubuntu 24.04

The release job runs on `ubuntu-24.04` on purpose. Its temporary sudoers rule uses command
argument wildcards. Classic sudo accepts them. sudo-rs on newer Ubuntu runners rejects them.
The rule is a fixture for disposable runners only. It lets the `devsy-sudo` account run a broad
root shell dispatch, `/bin/sh -c * devsy-install *`. It is not a production policy.

### Run it by hand

Use a disposable Ubuntu 24.04 Linux amd64 host with sshd and Docker. Provision the accounts first.
The `devsy-bash`, `devsy-zsh`, `devsy-fish`, and `devsy-sudo` accounts must exist and belong to the
`docker` group. Put the released provider binary at
`dist/build_linux_amd64_v1/devsy-provider-ssh-linux-amd64` and its checksums at `dist/checksums.txt`,
both relative to the repository root.

Unset `DEVSY_AGENT_URL`, `DEVSY_AGENT_BINARY`, and `DEVSY_AGENT_PREFER_DOWNLOAD` first.
Inherited overrides can disable the version check or select the wrong agent transport.

```
DEVSY_E2E_RELEASE_QUALIFICATION=true DEVSY_E2E_SHELL_MATRIX=true \
DEVSY_E2E_BINARY=/path/to/devsy-linux-amd64 DEVSY_E2E_CORE_TAG=v1.25.0-beta.8 \
DEVSY_E2E_CORE_SHA=f4d6039acf68bffcba9d1f9ce927cce144b7dc66 \
DEVSY_E2E_CORE_VERSION=v1.25.0-beta.8 \
DEVSY_E2E_PROVIDER_TAG=v1.1.47 \
DEVSY_E2E_PROVIDER_SHA=c53ba2742db162f587aa57fbc87572d19ec11cd7 \
go test -v -ginkgo.v -timeout 14400s \
  --ginkgo.label-filter='release-qualification || provider-command'
```

Run qualification in one process. Its fail-closed report state lives in that process.
It does not merge results from Ginkgo parallel nodes, so do not run it with parallel nodes.

When the candidate binary is missing, the run fails with `DEVSY_E2E_BINARY is required`.
A missing candidate never falls back to the compatibility core.

When the run ends, run `go run ./hack/ci cleanup-qualification-accounts` to remove the sudo rule.
The workflow does this in an `always()` step.

### Rules for the matrix

- Only the fixture accounts are provisioned. Never change a user's login shell, `SHELL`,
  `/etc/passwd`, or `.zshrc` to make a run pass.
- Every cell uses an agent path with a space, an apostrophe, a backslash, a dollar sign,
  and a Unicode character. Each remote helper quotes that path literally.
- Download and stdin transfer must deliver the candidate's exact bytes.
- The matrix keeps the command-boundary stream, stderr, and nonzero-exit coverage.
- A skipped or incomplete matrix or privilege scenario fails the report gate.

Each cell runs five agent scenarios. The privilege scenario runs once. It uses the `devsy-sudo`
account and a root-owned `0755` agent directory. It proves that an ordinary write is denied,
that a real sudo install produces a root-owned agent with the expected version and checksum,
and that workspace SSH still runs a live command.

## Report schema

The report is JSON. Its top-level `schemaVersion` is `1`, which is `qualification.SchemaVersion`.

- `pair`: `coreBinary`, `coreTag`, `coreSha`, `coreVersion`, `coreBinarySha256`, `providerRef`,
  `providerSha`, `providerBinary`, `providerBinarySha256`, and `harnessSha`.
- `cells`: one entry per shell and transport. Each has `shell`, `user`, `builtinSsh`, `workspace`,
  `agentDir`, `loginShell`, `scenarios`, `agentSha256`, and `cleanup`.
- `privilege`: the `devsy-sudo` run. It has `user`, `userUid`, `agentDir`, `directoryState`,
  `writeDenied`, `writeError`, `agentUid`, `agentVersion`, `agentSha256`, `liveCommand`,
  `outcome`, and `cleanup`.

`go run ./hack/ci verify-report` loads the report and checks it. The gate requires the pair to
match `CORE_SHA256`, all six cells to be present, every scenario to pass, cleanup to be `ok`,
and the privilege record to pass. A missing or failed item fails the gate.

## Add a shell or transport

1. Edit `Transports` in `e2e/qualification`. Each entry is a shell name and whether it uses
   built-in SSH.
2. Do not edit the matrix, the report checks, or the account provisioning. They all read
   `Transports` and `Shells()`.
3. If the workflow does not install the shell yet, add its package to the `apt-get install` lines
   in `tests.yaml`.

## Scope of the Desktop run

Extracting and qualifying the Desktop-bundled CLI is not native Desktop UI acceptance,
installation or update validation, or a customer host reproduction.
Stable release publication, supported-platform artifact smokes, reporter-equivalent Desktop
fresh and reopen traces, and customer confirmation remain separate release work.
