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
- `dots`: (Array of Tables, required) An array of dot entries, where each entry defines a symlink.
  - `source`: (String, required) The path to the source file or directory within your `dotfiles` directory, relative to the `dotfiles` path.
  - `target`: (String, required) The target path for the symlink. This can be either relative to your `$HOME` directory or an absolute path.
  - `hostname`: (Array of Strings, optional) An array of hostnames where this dot entry should be applied. If not specified, the entry will be applied to all hosts.

## Commands

### `link` (default)

Creates symlinks from the `source` files/directories in your `dotfiles` directory to the `target` locations specified in the configuration file.

- **Handling Conflicts**: If a file or directory already exists at the `target` location, Foondot will move the existing file/directory into your `dotfiles` directory before linking. If the source file/directory also exists, it appends `.conflict` to the name. For example, if `.config/program` already exists, it will be moved to `dotfiles/program.conflict`.
- **Removing Symlinks**: Foondot tries to clean up links when they are removed from the config or no longer active for your hostname. It does this by keeping track of all the links it has written.

### `sync`

Automatically synchronizes your dotfiles repository using Git. It follows a streamlined workflow:

1.  **Pull**: Performs a `git pull --rebase --autostash` to integrate remote changes while preserving local modifications.
2.  **Stage**: Automatically stages all changes in the dotfiles directory (`git add -A`).
3.  **Commit**: Generates a "smart" commit message based on the changed top-level folders and files (e.g., `Updated sway and mako, Added alacritty`).
4.  **Push**: Pushes the local commits to the remote tracking branch.

If a merge conflict occurs during the sync process, Foondot will abort and notify you to resolve it manually.

## Usage

Foondot uses a subcommand structure. Running it without a command defaults to `link`.

### Command-Line Options:

- `-f`: Force relinking and move conflicting files (applies to `link` command).
- `-c <path>`: Specify the location of an alternate configuration file.
- `-v`: Show the version and hostname.
- `-cc`: Enable color output.

### Examples:

- **Link dotfiles** (default behavior):

  ```bash
  foondot
  # or explicitly
  foondot link
  ```

- **Sync dotfiles with Git**:

  ```bash
  foondot sync
  ```

- **Force relink with a specific config**:
  ```bash
  foondot -f -c /path/to/myconfig.toml link
  ```

## Error Handling

Foondot provides informative error messages in case of issues.

- **Missing Configuration File:** If the main configuration file is missing, an empty one will be generated in `$HOME/.config/foondot.toml`.
- **Faulty Configuration:** If there are errors in the configuration file (e.g., invalid TOML syntax, missing required fields), Foondot will display an error message explaining the problem.
- **Git Errors:** The `sync` command will report errors if the directory is not a Git repository or if network/conflict issues occur during push/pull.
