### E2E tests

#### Prerequisites

Make sure you have ginkgo installed on your local machine:
```
go get github.com/onsi/ginkgo/ginkgo
```

#### Run all E2E test
```
# Install ginkgo and run in this directory
go test -v -ginkgo.v -timeout 3600s
```

#### Release qualification (explicit core/provider pair)

The default lifecycle suite downloads core v1.24.0 unless `DEVSY_E2E_BINARY` is set.
That run checks backward compatibility only and is not release qualification.

The qualification suite requires an explicit `DEVSY_E2E_BINARY`; missing candidate
metadata or host prerequisites fail rather than selecting the compatibility core.
Dispatch the `Tests` workflow with `core_tag` and `provider_tag` (default `v1.1.47`).
Two independent jobs qualify the standalone Linux amd64 CLI and the CLI extracted
from the Desktop candidate's `Devsy_linux_amd64.deb`. Each job verifies the published
asset digest, executes that binary's `--version`, records its SHA256, and checks that
core `e22be00` and provider `d31fed1` are ancestors of the selected release commits.
The provider binary and checksums come from the selected provider release.

The jobs run `release-qualification || provider-command` and upload
`release-qualification-<tag>-cli` and `release-qualification-<tag>-desktop` reports.
The report identifies the exact core/provider pair and harness commit, all six
shell/transport cells, five agent scenarios per cell, binary hashes, and cleanup.
All six cells use an agent path containing a space, apostrophe, backslash, dollar
sign, and Unicode character; every remote helper quotes the path literally.
One additional focused scenario uses the dedicated non-root `devsy-sudo` account
and a root-owned `0755` agent directory. It proves that an ordinary write is
denied, a real sudo installation produces a root-owned candidate agent with the
expected version/checksum, and workspace SSH still executes a live command.
A skipped or incomplete matrix or privilege scenario cannot pass the report gate.

To run by hand, use a disposable Linux amd64 host with local sshd, Docker, and
provisioned `devsy-bash`, `devsy-zsh`, `devsy-fish`, and `devsy-sudo` accounts in the
`docker` group. Use the workflow's provisioning on that owned host: only
`devsy-sudo` receives the validated, temporary sudoers rule for
`/bin/sh -c * devsy-install *`, with `NOPASSWD` and `SETENV` for the installer's
explicit dispatch. Remove `/etc/sudoers.d/devsy-e2e-qualification` after the run,
including failure paths, as the workflow's `always()` cleanup does.
Run from `e2e/`, after placing the released provider binary at
`dist/build_linux_amd64_v1/devsy-provider-ssh-linux-amd64` and its release checksums
at `dist/checksums.txt` relative to the repository root. Unset `DEVSY_AGENT_URL`,
`DEVSY_AGENT_BINARY`, and `DEVSY_AGENT_PREFER_DOWNLOAD`: inherited overrides can
disable the version check or select the wrong agent transport.

```
DEVSY_E2E_RELEASE_QUALIFICATION=true DEVSY_E2E_SHELL_MATRIX=true \
DEVSY_E2E_BINARY=/path/to/devsy-linux-amd64 DEVSY_E2E_CORE_TAG=v1.25.0-beta.3 \
DEVSY_E2E_CORE_SHA=fe22f9a1225d6c1547001755e94c95bb2cdbc8db \
DEVSY_E2E_CORE_VERSION=v1.25.0-beta.3 \
DEVSY_E2E_PROVIDER_TAG=v1.1.47 \
DEVSY_E2E_PROVIDER_SHA=c53ba2742db162f587aa57fbc87572d19ec11cd7 \
go test -v -ginkgo.v -timeout 14400s \
  --ginkgo.label-filter='release-qualification || provider-command'
```

Only disposable fixture accounts are provisioned; never change a user's login shell,
`SHELL`, `/etc/passwd`, or `.zshrc` to make qualification pass. Download and stdin
transfer must deliver the candidate's exact bytes. The matrix retains the existing
command-boundary stream, stderr, and nonzero-exit coverage.

Extracting and qualifying the Desktop-bundled CLI is not native Desktop UI
acceptance, installation/update validation, or a customer-host reproduction.
Stable release publication, supported-platform artifact smokes, reporter-equivalent
Desktop fresh/reopen traces, and customer confirmation remain separate release work.
