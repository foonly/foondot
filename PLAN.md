# Expand the functionality of the utility to support automatic syncing through Git.

This would be implemented by adding a 'sync' command. Since the utility only uses flags now, this would require adding a command parser.

The utility should detect if the dotfiles folder is a git repository, and exit with a message if not.

The idea would be that the sync command should be as automatic as possible, so it should stage files automatically and write reasonable commit messages. The messages should list all folders or files in the folder root that has changes in them.

An example of this would be if the root folder has folders for bash, sway, mako and kitty, and you would change a file in the sway and mako folder, add a alacritty folder and remove the kitty folder, the commit message should read:

```
Updated sway and mako, Added alacritty, Removed kitty
```

It should also try to pull all changes and push the commit created. In case of merge conflicts, it should try to resolve them automatically. If it fails, it should exit with a message indicating the conflict.

## Implementation Roadmap

### Phase 1: CLI Refactor

- Transition from global flags to subcommands using `flag.NewFlagSet`.
- Support `link` (current default) and `sync`.
- Maintain backward compatibility: running `foondot` without a command should still perform the linking process.

### Phase 2: Git Integration Module (`internal/git`)

- Create a new package for Git operations.
- **Repo Detection**: Verify if the dotfiles directory is a Git repository.
- **Status Analysis**: Use `git status --porcelain` to identify changed files.
- **Smart Commit Messaging**:
  1. Group changes by top-level directory/file name.
  2. Map Git status codes (`M`, `A`, `D`, `??`) to actions: "Updated", "Added", "Removed".
  3. Format lists into a human-readable string (e.g., "Updated sway and mako").

### Phase 3: Sync Workflow

1.  **Pull**: Perform `git pull --rebase` to integrate remote changes.
2.  **Add/Commit**: Stage all changes and commit with the generated smart message.
3.  **Push**: Push the local commit to the remote tracking branch.
4.  **Error Handling**:
    - Detect merge conflicts during `pull --rebase`.
    - Gracefully abort and notify the user if manual intervention is required.
    - Exit with a non-zero code and a descriptive error message if any step fails.

### Phase 4: Integration

- Update `cmd/foondot/main.go` to invoke the Git sync logic when the `sync` command is used.
- Ensure configuration is correctly loaded and the dotfiles path is resolved before Git operations begin.
