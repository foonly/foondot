# Foondot

Foondot is a utility that manages symlinks from a central repository, linking files and folders according to a configuration file. It is written in Go and statically linked, requiring no special dependencies.

## Configuration

The configuration file is in TOML format. By default, Foondot looks for a configuration file in `$HOME/.config/foondot.toml`. If the configuration file is missing, an empty one will be generated.

### Example Configuration File:

```toml
# Path to your dotfiles relative to your $HOME directory
dotfiles = "dotfiles"

# Enable color output
color = false

# A dot entry representing a symlink, `source` is relative to `dotfiles_dir`
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

## Symlink Management

Foondot creates symlinks from the `source` files/directories in your `dotfiles` directory to the `target` locations specified in the configuration file.

### Handling Conflicts

If a file or directory already exists at the `target` location, Foondot will move the existing file/directory into your `dotfiles` directory before linking. If the source file/directory also exists, it appends `.conflict` to the name. For example, if `.config/program` already exists, it will be moved to `dotfiles/program.conflict`.

### Removing Symlinks

Currently, Foondot does not automatically remove symlinks that are no longer defined in the configuration file. This feature is planned for a future release.

## Usage

Foondot is run from the command line.

### Command-Line Options:

- `-f` or `--force`: Force relinking and move conflicting files.
- `-c` or `--config`: Specify the location of an alternate configuration file.
- `-v` or `--version`: Show the version and hostname.
- `-cc` or `--color`: Enable color output.

### Examples:

- Run Foondot with the default configuration file:

  ```bash
  foondot
  ```

- Run Foondot with a specific configuration file:

  ```bash
  foondot -c /path/to/myconfig.toml
  ```

- Run Foondot with force relinking:

  ```bash
  foondot -f
  ```

## Error Handling

Foondot provides informative error messages in case of issues.

- **Missing Configuration File:** If the main configuration file is missing, an empty one will be generated in `$HOME/.config/foondot.toml`.
- **Faulty Configuration:** If there are errors in the configuration file (e.g., invalid TOML syntax, missing required fields), Foondot will display an error message explaining the problem.
- **Permission Errors:** Foondot may encounter permission errors when creating symlinks. Ensure that you have the necessary permissions to create symlinks in the target directories.

## Planned Features

- Automatically remove symlinks that are no longer defined in the configuration file.
