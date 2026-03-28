# Changelog

## 0.12.0 (2026-03-28)

#### Features
- git: Add sync strategy for git conflicts (9a82917)

#### Build System
- Add version.json (2a9e007)

## v0.11.0 (2026-03-01)

#### Features
- dots: add wildcard support for source paths (2666b1e)
- cli: change default subcommand to sync (a81e5a2)

#### Documentation
- explain wildcard support in source paths (651e703)
- git: improve documentation and refine log output (123291e)

### v0.10.2 (2026-02-23)

#### Bug Fixes
- cli: handle unknown commands and refine sync error message (af26d4a)

#### Refactor
- dots: move linking logic from main to dots package (1103344)

### v0.10.1 (2026-02-23)

### Misc
- Improve git command error handling (66d0dac)
- Use double dash in git ls-files (5f6610b)
- Improve commit message generation logic (0336d9c)

## v0.10.0 (2026-02-22)

#### Features
- cli: add sync command for git integration (cd5d0ee)

#### Documentation
- readme: document sync command and subcommand structure (70bc63a)
- add implementation roadmap for git sync (d9ef64c)
- dots: add comment to CleanTargets function (29affd8)
- Update README.md (e6bad7f)

#### Maintenance
- remove PLAN.md (32f0823)

### Misc
- Add build target (9bb4d74)

## v0.9.0 (2025-10-25)

#### Features
- Add CleanTargets to remove stale symlinks from DotsData (7f388fc)
- Add one string variant to printMessage (0621c71)
- Save symlink targets in datafile. (4486ac8)
- Add hostname filtering (741d995)
- Move config and print functions to separate files (dcbe951)
- Add hostname and config file checks (242a523)

#### Bug Fixes
- Return false on failed link removal in CleanTargets (8850616)
- Handle errors when removing symlink targets (d8c837c)

#### Refactor
- CleanTargets to use slices.DeleteFunc (5ab423e)
- file type constants to use single iota declaration (695b761)
- Restructure project into modular packages and update entrypoint (c7b96a7)
- move dotfile handling and utils to separate files (d27b706)
- Filter dotfiles by hostname before linking (f5ae1b6)

#### Documentation
- Fix typos in comments in CleanTargets function (c95d54d)
- Improve and expand Go doc comments for config functions (94c8d71)
- Clarify readConfig documentation (a9ca46c)
- Clarified some texts. (6d5b16a)
- Fix conflict handling description (d1c91bf)
- Update README with usage and configuration details (29fefc8)

## v0.8.0 (2025-10-21)

### v0.7.1 (2025-10-21)

#### Bug Fixes
- Create directories recursively (b445281)

#### Documentation
- Create getting started section. (e843fb8)

## v0.7.0 (2025-10-17)

#### Features
- Create default config file if it does not exist (53ed9bb)

### v0.6.1 (2025-10-17)

#### Bug Fixes
- Add handling of multiple .conflict files. (904dc0e)

#### Documentation
- Add color configuration option (e3b3136)

#### Build System
- Use tags in version (1c003ab)

## v0.6.0 (2025-10-17)

#### Features
- Add color support (24c4853)

## v0.5.0 (2025-10-17)

#### Features
- Added force flag and logic for it. (25ddcf9)

#### Continuous Integration
- Build with version flag (4205d21)

#### Maintenance
- Remove package.json (4a1e464)

### Doc
- Readme and example file (cbc7632)

### v0.4.1 (2025-10-16)

#### Features
- Add install target and example config (c64a1a4)

## v0.4.0 (2025-10-15)

#### Continuous Integration
- testing use of package.json for version tracking. (8e7cb88)

### Feature
- Moves target to dotfiles folder before linking it back. (6d033dd)

### Misc
- Rename dotsync to foondot (3c12b5f)
- Command line flags (39c8b7b)

## v0.2.0 (2024-09-05)

#### Maintenance
- clean up dependencies (35de1e5)

### Misc
- Changed config file name. (e5cc086)

## v0.1.0 (2024-09-04)

#### Features
- basic functionality now there (cfea347)
- Symlink creation (ac2b072)

### v0.0.2 (2024-09-04)

### v0.0.1 (2024-09-04)

#### Features
- read config file (655e9fb)

### Misc
- Makefile and workflow (c6245e2)
- Initial commit (bf5d0e1)

