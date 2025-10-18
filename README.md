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

### Getting started

If the target exists as a folder or file, but the source doesn't, the utility first moves it to the dotfiles folder, so you don't need to do that manually.
If both source and target is a file of folder, it will not do anything. But if used with the -f (force) flag, it will move the target to the dotfiles folder with an appanded .conflict suffix.
