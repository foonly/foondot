# Foondot Code Quality and Security Audit

Audit of foondot 0.12.0 (`59e8ebd`), updated 2026-09-29 after the first round of fixes.

## Summary

Foondot is small (about 1,000 lines of Go) and has few dependencies. `go vet` and `govulncheck` report nothing. The real risks are in the logic: `sync` publishes the whole dotfiles repository, and `link` deletes and moves files. The main problems found were:

- conflict handling that could act on the wrong repository or hang
- link cleanup that could remove more than it should
- a default command that committed and pushed without being asked

Those are fixed in `18b9648`, `89de62f` and `d75ca29`, and the fixes are covered by the first tests in the project. The rest of the findings are still open and listed below by priority.

Severity is given for a single-user tool that runs with your own permissions on your own config and dotfiles repository. The config file is trusted input. Repository contents are semi-trusted, because they may come from another machine through `sync`.

| # | Finding | Severity | Status |
|---|---------|----------|--------|
| 1 | `manual` strategy aborts the rebase in the wrong directory | High | Fixed `18b9648` |
| 2 | Conflict resolve loop can hang forever | High | Fixed `18b9648` |
| 3 | Absolute `target` paths are placed under `$HOME` | High | Open |
| 4 | Link cleanup removes links it doesn't own or shouldn't remove | High | Fixed `89de62f` |
| 5 | Running without a command syncs (commits and pushes everything) | High | Fixed `d75ca29` |
| 6 | Flags after the command are silently ignored | Medium | Fixed `d75ca29` |
| 7 | Wrong exit codes | Medium | Partly fixed `d75ca29` |
| 8 | Fragile parsing of git output lets the conflict-marker check be bypassed | Medium | Open |
| 9 | Filenames passed to git without `--` | Medium | Open |
| 10 | `sync_strategy` value is not checked | Medium | Open |
| 11 | Repository detection assumes a plain `.git` folder | Medium | Open |
| 12 | Garbled output when resolving conflicts | Low | Open |
| 13 | Filesystem errors are ignored during linking | Medium | Open |
| 14 | Backups and moved files end up in the synced repository | Medium | Open |
| 15 | Link tracking file is fragile | Low | Open |
| 16 | Git errors carry no detail | Low | Open |
| 17 | Structure: `os.Exit` in internal packages, global state | Medium | Open |
| 18 | Tests only cover the fixed areas | Medium | Partly fixed |
| 19 | Go style issues | Low | Open |
| 20 | Release workflow | Low | Open |
| 21 | README is out of date | Low | Partly fixed |

## Fixed

### 1. `manual` strategy aborts the rebase in the wrong directory

`resolveRebase` ran `git rebase --abort` without setting `cmd.Dir`, so it ran in the directory foondot was started from:

- The dotfiles repository was left in the middle of a rebase, with conflict markers in the files.
- If the current directory was another repository in the middle of a rebase, that rebase was aborted instead.

**Fix:** the abort now runs in the dotfiles repository, and if the abort fails, that error is returned. Test: `TestResolveRebaseManualAbortsInRepo`.

### 2. Conflict resolve loop can hang forever

Errors from `git rebase --continue` were ignored and the loop repeated while a rebase was in progress. If `continue` failed without leaving conflicts behind, the loop never ended. For example, with no git identity configured, the commit fails and the rebase stays stopped. That is a likely state on a freshly set-up machine.

**Fix:** each pass must resolve at least one conflict. Otherwise the last `continue` error is returned. Test: `TestResolveRebaseContinueFails`, which hangs on the old code.

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

## Open

### 3. Absolute `target` paths are placed under `$HOME`

**Where:** `internal/dots/dots.go:109`, `:135`

The README and the config comments say `target` can be absolute. But `path.Join(xdg.Home, "/etc/foo")` gives `$HOME/etc/foo`, so an absolute target creates directories and a link inside your home directory instead.

**Recommendation:** use the target as-is when `filepath.IsAbs(item.Target)` is true. Resolve it the same way in `handleDot` and `cleanTargets`, and put the logic in one shared helper so the two can't drift apart.

### 7. Wrong exit codes (partly fixed)

**Where:** `internal/dots/dots.go`

Unknown commands and unexpected arguments now exit with 2. `link` still always exits with 0, even when links failed or files couldn't be moved, which hides failures from scripts.

**Recommendation:** have `Link` return an error or a failure count, and exit non-zero when anything failed.

### 8. Fragile parsing of git output lets the conflict-marker check be bypassed

**Where:** `internal/git/git.go:197` (`getChanges`), `internal/utils/file.go:51`, `getConflictedFiles`

- `strings.TrimSpace` on the whole `git status --porcelain` output removes the leading space of the first line. ` M sub` is parsed as status `M` and path `ub`.
- Git quotes unusual filenames and escapes characters such as non-ASCII ones (e.g. `"\303\244"`). Only the quotes are stripped, so the resulting path doesn't exist.
- `ContainsConflictMarkers` returns `false` when it can't read a file. Together with the previous point, files with unusual names skip the conflict-marker check and can be committed and pushed with conflict markers in them.
- The same quoting breaks `git checkout --theirs` for such files, and the grouping in generated commit messages.

**Recommendation:**

- Use `git status --porcelain -z` and `git diff --name-only -z --diff-filter=U`, and split on NUL.
- Treat unreadable files as a failed check (or report them) instead of as clean.
- Anchor the marker check to the start of lines, since `=======` alone is common in Markdown and reStructuredText headings.

### 9. Filenames passed to git without `--`

**Where:** `internal/git/git.go:171`, `:177`

`git checkout --theirs <file>` and `git add <file>` take filenames from the repository. A file whose name starts with `-` is read as an option. Those files can come from another machine through `sync`.

**Recommendation:** add `"--"` before the filename in both commands.

### 10. `sync_strategy` value is not checked

**Where:** `internal/git/git.go:132`, `:161`, `internal/config/config.go`

Only `manual` and `remote` are compared against. Any other value, including a typo like `remot`, behaves like `local` and silently replaces remote changes with local ones.

**Recommendation:** reject values other than `manual`, `local` and `remote` when the config is loaded.

### 11. Repository detection assumes a plain `.git` folder

**Where:** `internal/git/git.go:99` (`isRepo`), `:107` (`isRebasing`)

- `isRebasing` looks for `<dotfiles>/.git/rebase-*`. That fails for worktrees and submodules, where `.git` is a file, and when the dotfiles folder is a subfolder of a repository. The code then reports "failed to pull" instead of applying the strategy.
- It also treats any error other than "not found" (e.g. permission denied) as "rebasing".
- `isRepo` accepts any folder inside a work tree. If `$HOME` is itself a repository, `git add -A` stages the entire home work tree, not just the dotfiles.

**Recommendation:**

- Use `git rev-parse --git-path rebase-merge` (and `rebase-apply`) to find the rebase state.
- Require `git rev-parse --show-toplevel` to equal the dotfiles folder, or pass the folder as a pathspec to `git add`.

### 12. Garbled output when resolving conflicts

**Where:** `internal/git/git.go:170`

`PrintMessage` uses only its first three arguments. `PrintMessage("Auto-resolving conflict in", file, "using", strategy, "version")` prints `Auto-resolving conflict in: <file> => using`. Also, `"...Applying strategy:"` followed by the value prints a double colon.

**Recommendation:** fix these calls. Better, switch the print helpers to format strings (see #19).

### 13. Filesystem errors are ignored during linking

**Where:** `internal/dots/dots.go:190`, `:201`, `:224`, `:278`

- If `MkdirAll` fails for the target's parent folder, nothing is reported.
- The result of `os.Remove(target)` in force mode is not checked.
- Moving an existing file into the repository (`os.Rename`) reports only success. It fails across filesystems (`EXDEV`), and the user then just sees "Source does not exist".
- "Linking" is printed before the result of `os.Symlink` is known.
- Folders are created with `os.ModePerm` (0777, reduced by umask).

**Recommendation:** report every error with its cause. Fall back to copy-and-remove when `Rename` fails with `EXDEV`. Use 0755 for created folders.

### 14. Backups and moved files end up in the synced repository

**Where:** `internal/dots/dots.go` (`prepareTargetSource`) together with `sync`

`link` moves existing target files into the dotfiles folder, and with `-f` it also leaves `*.conflict` backups there. `sync` then stages everything with `git add -A` and pushes. A file that happened to exist at a target path, and possibly contains secrets, is published by the next sync without being reviewed.

**Recommendation:**

- Write `.conflict` backups outside the repository (e.g. under `$XDG_DATA_HOME/foondot/backup`), or add `*.conflict*` to the repository's `.gitignore`.
- Consider having `sync` list new files before committing, or offer a `--dry-run`.

### 15. Link tracking file is fragile

**Where:** `internal/config/config.go:134`, `:157`

- Any error reading `dots.json`, not just "file not found", is treated as an empty list. The file is then overwritten and all tracking is lost, so old links are never cleaned up.
- The write isn't atomic. An interruption can leave broken JSON, which then blocks every later `link`.

**Recommendation:** only treat `fs.ErrNotExist` as empty, and write to a temporary file that is then renamed over the old one.

### 16. Git errors carry no detail

**Where:** `internal/git/git.go:248` and other command helpers

`stageAll`, `commit` and similar helpers discard git's stderr, so a failure shows up as `exit status 1`. For example, a missing git identity is reported only as "failed to commit: exit status 1".

**Recommendation:** use `CombinedOutput()` and include git's message in the returned error.

### 17. Structure: `os.Exit` in internal packages, global state

- `config` calls `os.Exit` from inside the package, and `dots` doesn't return errors, which makes both hard to test and to reuse.
- `config.Hostname`, `config.DotsData`, `utils.Color` and the use of `xdg.Home` are package-level state. The new tests have to save and restore them.

**Recommendation:** return errors to `cmd/foondot`, which decides the exit code. Pass the hostname, home and dotfiles folders, and the tracked links in a small state struct.

### 18. Tests only cover the fixed areas (partly fixed)

Tests now cover filtering, cleanup, link ownership and the conflict strategies. Still missing:

- `prepareTargetSource` and `doLink` (moves, `.conflict` naming, force mode)
- config loading
- parsing of `git status` output (easier once #8 is done)
- the `Sync` flow end to end
- command-line argument handling

### 19. Go style issues

- Doc comments use `/** ... */` with ` * ` prefixes. `go doc` shows the asterisks, and `git.go` already uses `//` comments.
- File types are plain integer constants. A named type (`type FileType int`) would let the compiler catch misuse.
- `PrintMessage` and `PrintError` pick a format from the number of arguments. Extra arguments are dropped (see #12), and calling them with no arguments panics.
- `path` is used where `path/filepath` belongs for filesystem paths.
- `os.IsNotExist` is used instead of `errors.Is(err, fs.ErrNotExist)`.
- `[]byte(data)` converts data that is already `[]byte`.
- When `os.Hostname()` fails, the hostname becomes `""` instead of keeping the `"unknown"` default (`cmd/foondot/main.go:49`).

### 20. Release workflow

**Where:** `.github/workflows/make-release.yaml`

- It uses Go `1.22.x`, while `go.mod` requires `1.24.0`. It works only because Go downloads the newer toolchain automatically.
- `go get .` can modify `go.mod`. Use `go mod download`.
- Actions are pinned to tags, not exact commits. That includes the third-party `ncipollo/release-action`, which runs with `contents: write`.
- There is no `go vet` or `go test` step before releasing, and no checksums are published for the binary.

**Recommendation:** read the Go version from `go.mod` (`go-version-file: go.mod`), pin actions to commit SHAs, add vet and test steps, and publish a `SHA256SUMS` file.

### 21. README is out of date (partly fixed)

The README no longer names a default command, and it describes the new cleanup and config-error behaviour. Still missing:

- documentation for `sync_strategy`
- the statement that conflicts always abort, which is wrong since the sync strategies were added
- the absolute-target claim, which stays wrong until #3 is fixed

## Not considered an issue

- **`..` in `source` or `target`:** the config file is written by the user and runs with their permissions, so paths outside `$HOME` or the dotfiles folder are a feature, not an escalation. Wildcard entries come from `os.ReadDir` and can't contain path separators.
- **Commit message injection:** filenames go into `git commit -m` as a single argument through `exec.Command` without a shell, so they can't inject commands.
- **Dependencies:** `govulncheck` found no known vulnerabilities (go-toml v2.2.4, xdg v0.5.3, x/sys v0.26.0).
