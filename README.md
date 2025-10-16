# foondot

A very simple dotfile sync utility written i Go.

## How it works?

Foondot makes symlinks from your dotfiles folder to where your config files need to be, that's pretty much it.
If the files or folders don't exist in the dotfiles folder, they are first moved there.

## How do I use it?

Use the example TOML configuration file to configure foondot. The format is very simple.

```
dotfiles = "dotfiles"

dots = [
    { source = "program", target = ".config/program" },
    { source = "bashrc", target = ".bashrc" },
]
```
