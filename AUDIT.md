# Foondot Code Quality and Security Audit

Audit of foondot 0.12.0 (`59e8ebd`), updated 2026-09-29 after five rounds of fixes. All findings are fixed.

## Summary

Foondot is small (about 1,000 lines of Go) and has few dependencies. `go vet` and `govulncheck` report nothing. The real risks are in the logic: `sync` publishes the whole dotfiles repository, and `link` deletes and moves files. The main problems found were:

- conflict handling that could act on the wrong repository or hang
- link cleanup that could remove more than it should
- a default command that committed and pushed without being asked

All 21 findings are fixed, and each fix is covered by tests that CI runs before every release. A few deliberate limitations are listed at the end.

Severity is given for a single-user tool that runs with your own permissions on your own config and dotfiles repository. The config file is trusted input. Repository contents are semi-trusted, because they may come from another machine through `sync`.

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | `manual` strategy aborts the rebase in the wrong directory | High | Fixed `9a27749` |
| 2 | Conflict resolve loop can hang forever | High | Fixed `9a27749` |
| 3 | Absolute `target` paths are placed under `$HOME` | High | Fixed `7e764a0` |
| 4 | Link cleanup removes links it doesn't own or shouldn't remove | High | Fixed `96f9b70` |
| 5 | Running without a command syncs (commits and pushes everything) | High | Fixed `bb19492` |
| 6 | Flags after the command are silently ignored | Medium | Fixed `bb19492` |
| 7 | Wrong exit codes | Medium | Fixed `bb19492`, `d1c5c35` |
| 8 | Fragile parsing of git output lets the conflict-marker check be bypassed | Medium | Fixed `f470d9d` |
| 9 | Filenames passed to git without `--` | Medium | Fixed `f470d9d` |
| 10 | `sync_strategy` value is not checked | Medium | Fixed `db24da6` |
| 11 | Repository detection assumes a plain `.git` folder | Medium | Fixed `8e40d72` |
| 12 | Garbled output when resolving conflicts | Low | Fixed `2264e44` |
| 13 | Filesystem errors are ignored during linking | Medium | Fixed `0934a34` |
| 14 | Backups and moved files end up in the synced repository | Medium | Fixed `0934a34`, `d7347e1` |
| 15 | Link tracking file is fragile | Low | Fixed `d986b3b` |
| 16 | Git errors carry no detail | Low | Fixed `2264e44` |
| 17 | Structure: `os.Exit` in internal packages, global state | Medium | Fixed `8b442bc` |
| 18 | Tests only cover the fixed areas | Medium | Fixed `abf4639` |
| 19 | Go style issues | Low | Fixed `8b442bc` |
| 20 | Release workflow | Low | Fixed `bdfd8b5` |
| 21 | README is out of date | Low | Fixed `bb19492`, `db24da6` |

## Fixed

### 1. `manual` strategy aborts the rebase in the wrong directory

`resolveRebase` ran `git rebase --abort` without setting `cmd.Dir`, so it ran in the directory foondot was started from:

- The dotfiles repository was left in the middle of a rebase, with conflict markers in the files.
- If the current directory was another repository in the middle of a rebase, that rebase was aborted instead.

**Fix:** the abort now runs in the dotfiles repository, and if the abort fails, that error is returned. Test: `TestResolveRebaseManualAbortsInRepo`.

### 2. Conflict resolve loop can hang forever

Errors from `git rebase --continue` were ignored and the loop repeated while a rebase was in progress. If `continue` failed without leaving conflicts behind, the loop never ended. For example, with no git identity configured, the commit fails and the rebase stays stopped. That is a likely state on a freshly set-up machine.

**Fix:** each pass must resolve at least one conflict. Otherwise the last `continue` error is returned. Test: `TestResolveRebaseContinueFails`, which hangs on the old code.

### 3. Absolute `target` paths are placed under `$HOME`

The README and the config comments say `target` can be absolute. But `path.Join(xdg.Home, "/etc/foo")` gives `$HOME/etc/foo`, so an absolute target created directories and a link inside your home directory instead.

**Fix:** a shared `targetPath` helper uses absolute targets as-is, both when linking and during cleanup. Tests: `TestTargetPath`, `TestLinkAbsoluteTarget`.

### 4. Link cleanup removes links it doesn't own or shouldn't remove

`cleanTargets` removed every tracked symlink that was no longer in the config:

- A link the user had since replaced with their own symlink was deleted.
- If a wildcard folder (`source = "dir/*"`) couldn't be read, its entries disappeared from the list and all their links were removed.
- The TOML decoder ignored unknown keys, so a typo like `dot = [...]` produced an empty list and every managed link was removed.

**Fix:**

- Only links that point into the dotfiles folder are removed. Other links are just no longer tracked.
- Cleanup is skipped when a wildcard source can't be read.
- Unknown keys in the config file are now an error that shows the offending line.

Tests: `TestCleanTargets`, `TestPointsInto`, `TestFilterDots*`.

### 5. Running without a command syncs

Bare `foondot` ran `sync`, which does `git add -A`, commits and pushes. The usage text claimed the default was `link`. This made publishing everything in the repository the easiest thing to trigger by accident (see also #14).

**Fix:** without a command, foondot prints usage and exits without reading the config or touching git. This is a breaking change.

### 6. Flags after the command are silently ignored

Go's standard `flag` package stops at the first argument that isn't a flag, so `foondot link -f` ran without force and without any warning.

**Fix:** the remaining arguments are parsed again after the command. Leftover arguments are an error.

### 7. Wrong exit codes

Unknown commands exited with 0, and `link` always exited with 0, even when links failed or files couldn't be moved. That hid failures from scripts.

**Fix:**

- Unknown commands and unexpected arguments exit with 2.
- `link` exits with 1 when any dotfile or old link failed, and says how many. Dotfiles that are already linked count as success.
- A target that's a symlink to another location used to be skipped silently. It is now reported, with a hint to use `-f`.

Tests: `TestLinkReturnsFailures`, `TestHandleDotExistingLinks`, `TestHandleDotMissingSource`.

### 8. Fragile parsing of git output lets the conflict-marker check be bypassed

- `strings.TrimSpace` on the whole `git status --porcelain` output removed the leading space of the first line. ` M sub` was parsed as status `M` and path `ub`.
- Git quotes unusual filenames and escapes characters such as non-ASCII ones (e.g. `"\303\244"`). Only the quotes were stripped, so the resulting path didn't exist.
- `ContainsConflictMarkers` returned `false` when it couldn't read a file. Together with the previous point, files with unusual names skipped the conflict-marker check and could be committed and pushed with conflict markers in them.
- The same quoting broke `git checkout --theirs` for such files, and the grouping in generated commit messages.

**Fix:**

- `git status` and `git diff` are read with `-z`, and renames carry their old path in a separate field.
- A file that can't be read now fails the check. Non-regular files, such as submodules and symlinks, are skipped.
- Markers only count at the start of a line, so a Markdown `=======` heading is no longer reported.

Tests: `TestGetChanges`, `TestCheckConflictMarkers`, `TestContainsConflictMarkers*`, `TestResolveRebaseUnusualFilenames`.

### 9. Filenames passed to git without `--`

`git checkout --theirs <file>` and `git add <file>` took filenames from the repository, so a file whose name starts with `-` was read as an option. Those files can come from another machine through `sync`.

**Fix:** both commands pass `--` before the filename. Test: `TestResolveRebaseUnusualFilenames` (files named `--ours` and `-f`).

### 10. `sync_strategy` value is not checked

Only `manual` and `remote` were compared against. Any other value, including a typo like `remot`, behaved like `local` and silently replaced remote changes with local ones.

**Fix:** values other than `manual`, `local` and `remote` are rejected when the config is loaded. Test: `TestValidateConfigSyncStrategy`.

### 11. Repository detection assumes a plain `.git` folder

- `isRebasing` looked for `<dotfiles>/.git/rebase-*`. That fails for worktrees and submodules, where `.git` is a file, and when the dotfiles folder is a subfolder of a repository. In a worktree, it even reported a rebase when there was none.
- It treated any error other than "not found" (e.g. permission denied) as "rebasing".
- `isRepo` accepted any folder inside a work tree. If `$HOME` was itself a repository, `git add -A` staged the entire home work tree, not just the dotfiles. In a subfolder, the conflict-marker check also looked for files in the wrong place.

**Fix:**

- `isRebasing` asks git for the location of its rebase state (`git rev-parse --git-path`).
- `sync` requires the dotfiles folder to be the top level of its own repository, comparing paths with symlinks resolved. This is a breaking change for anyone who kept dotfiles in a subfolder of a larger repository, a setup that didn't work correctly before either.

Tests: `TestCheckRepo`, `TestIsRebasingWorktree`.

### 12. Garbled output when resolving conflicts

`PrintMessage` uses only its first three arguments. `PrintMessage("Auto-resolving conflict in", file, "using", strategy, "version")` printed `Auto-resolving conflict in: <file> => using`. Also, `"...Applying strategy:"` followed by the value printed a double colon.

**Fix:** the output now reads `Auto-resolving conflict using local version: <file>`. The print helpers now have fixed parameters, so this can't happen again (see #19).

### 13. Filesystem errors are ignored during linking

- If `MkdirAll` failed for the target's parent folder, nothing was reported.
- The result of `os.Remove(target)` in force mode was not checked.
- Moving an existing file into the repository (`os.Rename`) only reported success. On failure, the user just saw "Source does not exist".
- "Linking" was printed before the result of `os.Symlink` was known.
- Folders were created with `os.ModePerm` (0777, reduced by umask).

**Fix:**

- `prepareTargetSource` now returns an error, and the entry is skipped with the cause, e.g. `~/blocker is not a directory`.
- "Linking" is printed only after the link exists.
- Folders are created with 0755.

Moves across filesystems still fail (`EXDEV`), but the error is now reported instead of hidden. A copy fallback wasn't added, since the dotfiles folder, targets and backup folder are normally all under `$HOME`.

Tests: `TestHandleDot*`, `TestPrepareTargetSourceReportsErrors`.

### 14. Backups and moved files end up in the synced repository

- With `-f`, `link` moved an existing target to `<source>.conflict` inside the dotfiles folder, and the next `sync` committed and pushed it.
- When a target exists but its source doesn't, `link` moves the target into the dotfiles folder. That's how you add an existing file to your dotfiles. But `sync` stages everything with `git add -A` and pushes, so a file that happened to exist at a target path, possibly containing secrets, was published by the next sync with no way to review it first.

**Fix:**

- Backups are written to `$XDG_DATA_HOME/foondot/backup/<target path>`, outside the repository, with a number appended when a backup already exists (`0934a34`).
- `foondot sync -n` lists what `sync` would commit and push, including untracked files, and runs the conflict-marker check, without pulling, staging, committing or pushing anything (`d7347e1`). `-n` with `link` is a usage error, so it can't link without warning. The README recommends the dry run after `link` has moved files.
- Moving targets into the dotfiles folder is intended and unchanged.

Tests: `TestHandleDotForceBacksUpOutsideDotfiles`, `TestSyncDryRun*`, `TestDescribeStatus`.

### 15. Link tracking file is fragile

- Any error reading `dots.json`, not just "file not found", was treated as an empty list. The file was then overwritten and all tracking was lost, so old links were never cleaned up.
- The write wasn't atomic. An interruption could leave broken JSON, which then blocked every later `link`.

**Fix:** only a missing file counts as empty, and other errors stop with a message. The file is written to a temporary file that is renamed over the old one. Links that already exist and point to the right source are tracked again, so a lost `dots.json` recovers on the next `link`. Tests: `TestReadDotsData`, `TestWriteFileAtomic*`, `TestHandleDotExistingLinks`.

### 16. Git errors carry no detail

`stageAll`, `commit` and similar helpers discarded git's stderr, so a failure showed up as `exit status 1`. For example, a missing git identity was reported only as "failed to commit: exit status 1".

**Fix:** all git commands except `pull` and `push`, which stream their output to the terminal, now include git's message in the returned error. Tests: `TestCommitErrorIncludesGitMessage`, `TestGitOutputErrorIncludesStderr`.

### 17. Structure: `os.Exit` in internal packages, global state

- `config` called `os.Exit` from inside the package, which made it hard to test and to reuse.
- `config.Hostname`, `config.Version`, `config.DotsData`, `utils.Color` and the use of `xdg.Home` were package-level state. Tests had to save and restore them.

**Fix:**

- `config` returns errors, and only `cmd/foondot` decides exit codes.
- Linking state lives in a `linker` struct built from `dots.Options` (home, hostname, data folder, force). The tracked links are passed in and out explicitly.
- `git.Sync` takes the dotfiles folder and `git.Options`.
- `xdg` and `os.Hostname` are only read in `Execute`, which calls a `run(args, env)` function that tests can drive with temporary folders.
- `utils.Color` remains global on purpose: it's a process-wide output setting that's set once before anything is printed.

### 18. Tests only cover the fixed areas

**Fix:** besides the tests added with each fix, there are now tests for:

- loading and decoding the config file (`TestReadConfig*`, `TestCreateDefaultConfig`)
- `Sync` end to end against a real remote: pushing, pulling, nothing to sync, conflict markers, the dry run (`TestSync*`)
- command-line handling: usage errors, help, version, default config, flags after the command, exit codes (`TestRun*`)

### 19. Go style issues

- Doc comments used `/** ... */` with ` * ` prefixes.
- File types were plain integer constants.
- `PrintMessage` and `PrintError` picked a format from the number of arguments, silently dropping extras (this caused #12), and panicked when called without arguments.
- `path` was used for filesystem paths.
- `os.IsNotExist` and redundant `[]byte(data)` conversions were used.
- When `os.Hostname()` failed, the hostname became `""`.

**Fix:**

- All comments are Go doc comments.
- File types are a named `FileType`.
- The print helpers have fixed parameters (`PrintValue`, `PrintChange`, `PrintError`, `PrintErrorCause`, and so on).
- `path/filepath` is used throughout, along with `errors.Is(err, fs.ErrNotExist)`.
- The hostname falls back to `"unknown"`.

### 20. Release workflow

- It used Go `1.22.x`, while `go.mod` requires `1.24.0`. It only worked because Go downloads the newer toolchain automatically.
- `go get .` can modify `go.mod`.
- Actions were pinned to tags, not exact commits. That included the third-party `ncipollo/release-action`, which runs with `contents: write`.
- There was no `go vet` or `go test` step before releasing, and no checksums were published for the binary.

**Fix:**

- The Go version is read from `go.mod`, and dependencies are fetched with `go mod download`.
- `go vet` and `go test` run before the build.
- The binary is built statically (`CGO_ENABLED=0`, `-trimpath`), and `SHA256SUMS` is published with it.
- Actions are pinned to commits: checkout v4.4.0, setup-go v5.6.0, release-action v1.21.0. Newer major versions of checkout and setup-go (v7) exist, so upgrading them is a separate decision.

### 21. README is out of date

The README named `link` as the default in one place and `sync` in another. It didn't document `sync_strategy`, and it still said conflicts always abort.

**Fix:** the README names no default command, documents `sync_strategy` and each strategy, and describes the cleanup, config-error and conflict-marker behaviour.

## Known limitations

These were considered and left as they are:

- **Moves across filesystems fail** (`EXDEV`) when the dotfiles folder, a target or the backup folder are on different filesystems. The error is reported, but there is no copy fallback, since all three are normally under `$HOME` (#13).
- **Newer GitHub Action majors:** actions/checkout and actions/setup-go are pinned to the latest v4 and v5 releases. v7 of both is available, so upgrading is a separate decision (#20).
- **Global color setting:** `utils.Color` is the one remaining package-level variable (#17).

## Not considered an issue

- **`..` in `source` or `target`:** the config file is written by the user and runs with their permissions, so paths outside `$HOME` or the dotfiles folder are a feature, not an escalation. Wildcard entries come from `os.ReadDir` and can't contain path separators.
- **Commit message injection:** filenames go into `git commit -m` as a single argument through `exec.Command` without a shell, so they can't inject commands.
- **Dependencies:** `govulncheck` found no known vulnerabilities (go-toml v2.2.4, xdg v0.5.3, x/sys v0.26.0).
