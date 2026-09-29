# Foondot

Foondot is a utility that manages symlinks from a local repository, linking files and folders according to a configuration file. It also features a built-in sync command to automatically pull, commit, and push changes using Git.

Foondot is written in Go and statically linked, requiring no special dependencies.

## Configuration

The configuration file is in TOML format. By default, Foondot looks for a configuration file in `$HOME/.config/foondot.toml`. If the configuration file is missing, an empty one will be generated.

### Example Configuration File:

```toml
# Path to your dotfiles relative to your $HOME directory
dotfiles = "dotfiles"

# Enable color output
color = false

# Strategy for resolving git conflicts (manual, local, remote)
sync_strategy = "manual"

# A dot entry representing a symlink, `source` is relative to `dotfiles`
# and `target` shall be relative to $HOME directory or absolute.
dots = [
    { source = "program", target = ".config/program", hostname = ["myhost"] },
    { source = "bashrc", target = ".bashrc" },
]
```

### Configuration Options:

- `dotfiles`: (String, required) The path to your dotfiles directory, relative to your `$HOME` directory. This directory should contain the source files and directories that you want to symlink.
- `color`: (Boolean, optional) Enable color output in the console. Defaults to `false`.
- `sync_strategy`: (String, optional) How `sync` resolves conflicts when pulling. One of `manual`, `local` or `remote`. Defaults to `manual`.
- `dots`: (Array of Tables, required) An array of dot entries, where each entry defines a symlink.
  - `source`: (String, required) The path to the source file or directory within your `dotfiles` directory, relative to the `dotfiles` path. If the source path ends in /\*, the individual files in the folder are linked separately.
  - `target`: (String, required) The target path for the symlink. This can be either relative to your `$HOME` directory or an absolute path.
  - `hostname`: (Array of Strings, optional) An array of hostnames where this dot entry should be applied. If not specified, the entry will be applied to all hosts.

## Commands

### `link`

Creates symlinks from the `source` files/directories in your `dotfiles` directory to the `target` locations specified in the configuration file.

- **Handling Conflicts**: If a file or directory already exists at the `target` location, Foondot will move the existing file/directory into your `dotfiles` directory before linking. If the source file/directory also exists, Foondot skips the entry, unless you use `-f`. With `-f`, the existing target is moved to a backup folder outside your `dotfiles` directory, so it is never synced: `$XDG_DATA_HOME/foondot/backup/` (usually `~/.local/share/foondot/backup/`), followed by the full target path. For example, `~/.config/program` is moved to `~/.local/share/foondot/backup/home/<user>/.config/program`. If a backup already exists, a number is appended.
- **Removing Symlinks**: Foondot tries to clean up links when they are removed from the config or no longer active for your hostname. It does this by keeping track of all the links it has written. Links that no longer point into your `dotfiles` directory are left alone, and cleanup is skipped if a wildcard source can't be read.
- **Using Wildcards**: If the source path ends in /\*, the individual files in the folder are linked to the target location. This is useful if you want to combine different source paths into the same target. Note however that if using this, you need to re-link if you add or remove files or folders.

### `sync`

Automatically synchronizes your dotfiles repository using Git. The `dotfiles` directory must be the top level of its own Git repository, not a subdirectory of a larger one. It follows a streamlined workflow:

1.  **Pull**: Performs a `git pull --rebase --autostash` to integrate remote changes while preserving local modifications.
2.  **Stage**: Automatically stages all changes in the dotfiles directory (`git add -A`).
3.  **Commit**: Generates a "smart" commit message based on the changed top-level folders and files (e.g., `Updated sway and mako, Added alacritty`).
4.  **Push**: Pushes the local commits to the remote tracking branch.

If a conflict occurs while pulling, Foondot applies the configured `sync_strategy`:

- `manual`: Abort the rebase and let you resolve the conflict yourself.
- `local`: Keep your local version of each conflicted file.
- `remote`: Keep the remote version of each conflicted file.

Before committing, Foondot also refuses to sync if any changed file contains git conflict markers.

Since `sync` commits every change in your `dotfiles` directory, including new files, use `foondot sync -n` to review what would be published first. This is especially useful after `link` has moved existing files into your `dotfiles` directory.

## Usage

Foondot uses a subcommand structure. Running it without a command only prints the version, hostname and usage, and does nothing else. Flags can be given before or after the command.

### Command-Line Options:

- `-f`: Force relinking and move conflicting files to the backup folder (applies to `link` command).
- `-n`: Show what `sync` would commit and push, without pulling, staging, committing or pushing anything (applies to `sync` command).
- `-c <path>`: Specify the location of an alternate configuration file.
- `-v`: Show the version and hostname.
- `-cc`: Enable color output.

### Examples:

- **Link dotfiles**:

  ```bash
  foondot link
  ```

- **Sync dotfiles with Git**:

  ```bash
  foondot sync
  ```

- **Review what sync would publish**:

  ```bash
  foondot sync -n
  ```

- **Force relink with a specific config**:
  ```bash
  foondot -f -c /path/to/myconfig.toml link
  ```

## Error Handling

Foondot provides informative error messages in case of issues.

- **Missing Configuration File:** If the main configuration file is missing, an empty one will be generated in `$HOME/.config/foondot.toml`.
- **Faulty Configuration:** If there are errors in the configuration file (e.g., invalid TOML syntax or unknown keys), Foondot will display an error message explaining the problem.
- **Git Errors:** The `sync` command will report errors if the directory is not the top level of a Git repository or if network/conflict issues occur during push/pull.
- **Linking Errors:** The `link` command reports each dotfile it couldn't link and why, and exits with a non-zero status if any dotfile or old link failed. Dotfiles that are already linked count as success.
