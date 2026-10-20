# SSH bootstrap regression verification

The provider builds one login-shell-safe `/bin/sh -c` launcher before transport
selection. Its original multiline script is one argument; stdin remains reserved
for the bootstrap handshake, binary injection, and the live tunnel. No SCP or
stderr-matching fallback remains.

## Revisions and environment

- Reported provider v1.1.44: `e37a6c68a695d9d1d3806b2d42a755750816f60e`.
- Initial provider main: `9c5c3ff673c364818d31627d45f365053100fa4b`.
- Final base refreshed to `839369fc438c4616bace5ee15994ab42b0fd0773`; upstream
  changed only the shfmt hook version. The isolated branch was rebased cleanly.
- Original core inspected by the coordinated core change:
  `1f47cd185` (v1.24.0).
- Linux qualification: Ubuntu aarch64 container with actual OpenSSH and separate
  non-root passwd accounts using Bash, Zsh, Fish, Dash, and BusyBox Ash.
- Local unit tests execute installed Bash, Zsh, and Fish on macOS arm64. These
  shell tests are distinct from the Linux SSH qualification.
- The provider's core module remains pinned to v1.23.2. No API upgrade was needed.
  The existing full-workspace E2E download is pinned to core v1.24.0, with
  `DEVSY_E2E_BINARY` selecting an explicitly supplied local core executable.

## Reproduced baseline

The original core bootstrap and original provider passed with Bash through both
transports. External Zsh failed with `command not found: sh -c`; external Fish
failed with unsupported assignment syntax. The built-in transport failed with
exit 127 and lost the useful remote stderr diagnostics.

## Implemented regression checks

- Exact launcher argument boundaries and script byte round trips through real
  Bash, Zsh, and Fish, including empty input, newlines, apostrophes, backslashes,
  dollar signs, backticks, parentheses, UTF-8, and substitution sentinels.
- NUL rejection before remote execution.
- External and built-in transport byte-preserving stdin/stdout, separate stderr,
  wrapped exit-status errors, and cancellation.
- Built-in cancellation with an idle file pipe and an idle `io.PipeReader`, with
  input-copy completion joined before returning. Non-file closable input belongs
  to the session; unsupported potentially unbounded readers are rejected.
- Local stdin read failures remain errors even when the remote consumer exits
  successfully after EOF. Independent review identified and verified this fix.
- Windows uses the actual redirected pipe handle, duplicated for session-owned
  cancellation rather than opening a console. Native Windows CI covers redirected
  process stdin plus an in-process SSH session's pipe bytes and idle cancellation.
  Native Windows execution is NOT RUN locally; the final Windows suite
  cross-compiled successfully and Windows-target lint reported zero issues.
- Literal agent directory and Docker executable paths containing shell syntax.
- Linux CI provisions Bash/Zsh/Fish passwd accounts and exercises all three with
  both transports using binary streams and an exit-37 command. Missing required
  shells fail unit tests in CI.

## Coordinated Linux observations

The initial corrected provider and corrected core bootstrap passed all 30 cells:
five passwd shells, both transports, and cold/warm/forced-mismatch injection.
Each cell preserved a 4 KiB all-byte payload and a follow-on exchange sent only
after receiving output. Warm runs requested no replacement binary; cold and
forced-mismatch runs requested exactly one. This uses a synthetic agent payload
and must not be confused with the real-agent check below.

The subsequent `final5` build passed all 30 randomized/hostile-path synthetic
cells and all 10 real-agent shell/transport combinations. The latter started the
actual 207 MB Linux Devsy agent, received its `SSH-2.0-Go` banner, and completed
two sequential live commands per combination. SIGTERM ended both provider
transports in under half a second, left no local SSH child, and left no remote
Devsy agent. Generic non-PTY remote `sleep` commands survive connection closure;
arbitrary remote process reaping is outside this SSH transport contract.

The final5 binary SHA256 was
`7e18087de9fbc9137a7a561c6fce200b7fe50d523fd99db08b6e8ce7b27015c7`.
Later stdin-source-error and Windows fixes were independently reviewed. The
reviewed final-source binary SHA256 is
`39468e46550ddf4a9ea3a1033cae10ef02d0feabcce42ad7b89a7f88f40ba989`.
Its production diff fingerprint (commands, SSH implementation, module files) is
`fde9ee3eceaba52ab03246f0d59151c7070c967168edbce6b63a04ec1ddef1a0`.
The final rerun passed:

- Fixed core + reviewed provider: all 30 synthetic cold/warm/mismatch cells.
- Original core v1.24.0 + reviewed provider: Bash/Zsh/Fish through both transports,
  all six cold injection and live follow-on cells.
- Actual Linux Devsy agent: all ten shell/transport combinations, two live
  commands per session.
- SIGTERM after the actual agent banner: external 0.459 s, built-in 0.405 s;
  no local SSH child and no remote Devsy agent remained.
- Initialization through all five shells and both transports: ten passes.
- Exact stdout, separate stderr, and exit 37 through all five shells and both
  transports: ten passes.

The artifact was built before the final base's hook-only update; production
source is unchanged. All observations are Linux aarch64 SSH qualification,
not Windows or GUI acceptance.

## Local validation

- CI-parity unit suite: `go test -race -covermode=atomic . ./cmd/... ./hack/... ./pkg/...` PASS.
- SSH package statement coverage: 78.1%.
- E2E package compilation without host setup: PASS.
- Diff-scoped golangci-lint with macOS and Windows build contexts: zero issues.
- `prek run --all-files`: all applicable hooks PASS on the refreshed base.
- Local GoReleaser snapshot with publishing disabled: all five configured target
  builds PASS. This is cross-compilation, not native Windows execution.
- Provider YAML generation with genuine snapshot checksums: PASS. The existing
  `${SSH_PROVIDER} init` and `${SSH_PROVIDER} command` interface is unchanged.
  No generated descriptor was added to the source tree.

## First CI run and test correction

On provider head `c0c159aaed22ea789515380406e655b945006f29`, native Windows pipe
checks, lint, pre-commit, unit tests, snapshot builds, and Linux shell-account
provisioning passed. Linux E2E reported 14 passes, one failure, and three skips:
the existing failing-command assertion required Bash's English phrase
`command not found`, while Dash correctly emitted
`/bin/sh: 1: not-a-command: not found`. The follow-up changes only that assertion
to require exit 127, empty stdout, and separate nonempty stderr containing the
command name. The three subsequent ordered whole-workspace CLI checks were
skipped on that first run and must not be reported as acceptance passes.

The production source fingerprint and reviewed binary above remain unchanged;
the final-head CI and reviewer gates must run again after this test-only fix.

## Review gates

Local CodeRabbit authentication was verified, but automatic approval review
rejected the review invocation because it uploads uncommitted and untracked
private source. That gate is BLOCKED pending explicit user authorization for the
concrete diff. No workaround or indirect upload was attempted.

CI and final-SHA Greptile/CodeRabbit reviews are pending draft PR publication and
must not be reported as passed. No merge, tag, release, or customer reply is part
of this change.
