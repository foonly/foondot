package dots

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
)

// Options control how Link works.
type Options struct {
	// Home is the home directory, the base for the dotfiles folder and relative targets.
	Home string
	// Hostname is the current hostname, for dots limited to specific hosts.
	Hostname string
	// DataDir is the folder for foondot's data: the tracked links and backups.
	DataDir string
	// Force replaces existing links and moves existing targets to the backup folder.
	Force bool
}

// linker holds the state of a single Link run.
type linker struct {
	home        string
	dotfilesDir string
	hostname    string
	backupDir   string
	force       bool
	// tracked lists the targets of all links created by foondot.
	tracked []string
}

// Link orchestrates the linking process for dotfiles. It reads the tracked
// links, filters dotfiles based on the current hostname, cleans up obsolete
// links, links each dotfile and persists the tracked links. An error is
// returned if any dotfile couldn't be linked or any old link couldn't be removed.
func Link(cfg config.Config, opts Options) error {
	tracked, err := config.ReadDotsData(opts.DataDir)
	if err != nil {
		return fmt.Errorf("couldn't read tracked links: %w", err)
	}

	l := &linker{
		home:        opts.Home,
		dotfilesDir: cfg.DotfilesDir(opts.Home),
		hostname:    opts.Hostname,
		backupDir:   config.BackupDir(opts.DataDir),
		force:       opts.Force,
		tracked:     tracked,
	}

	dotFiles, complete := l.filterDots(cfg.Dots)

	var failures []string
	if complete {
		if !l.cleanTargets(dotFiles) {
			failures = append(failures, "some old links couldn't be removed")
		}
	} else {
		utils.PrintWarning("Some sources could not be read, skipping removal of old links")
		failures = append(failures, "some sources couldn't be read")
	}

	numberLinked := 0
	numberFailed := 0
	for _, element := range dotFiles {
		linked, err := l.handleDot(element)
		if err != nil {
			utils.PrintErrorCause("Couldn't link", l.targetPath(element.Target), err)
			numberFailed++
		} else if linked {
			numberLinked++
		}
	}
	if numberFailed > 0 {
		failures = append(failures, fmt.Sprintf("%d of %d dotfiles couldn't be linked", numberFailed, len(dotFiles)))
	}

	if err := config.WriteDotsData(opts.DataDir, l.tracked); err != nil {
		utils.PrintErrorCause("Couldn't save tracked links", opts.DataDir, err)
		failures = append(failures, "tracked links couldn't be saved")
	}

	if l.force {
		utils.PrintMessage("Force mode enabled")
	}
	if numberLinked == 0 {
		utils.PrintMessage("No new dotfiles linked.")
	} else if numberLinked == len(dotFiles) {
		utils.PrintMessage(fmt.Sprintf("All %d dotfiles linked.", len(dotFiles)))
	} else {
		utils.PrintMessage(fmt.Sprintf("%d of %d dotfiles linked.", numberLinked, len(dotFiles)))
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, ", "))
	}
	return nil
}

// filterDots filters dotfile items by hostname and expands wildcard sources.
// An item with hostnames is only included if the current hostname is one of
// them. A source ending in "/*" is expanded to one item per entry in that
// folder. The returned bool is false if a wildcard source couldn't be read,
// which means the returned list is incomplete.
func (l *linker) filterDots(dots []config.Item) ([]config.Item, bool) {
	newDots := []config.Item{}
	complete := true
	for _, dot := range dots {
		if len(dot.Hostname) > 0 && !slices.Contains(dot.Hostname, l.hostname) {
			continue
		}
		if before, ok := strings.CutSuffix(dot.Source, "/*"); ok {
			sourcePath := filepath.Join(l.dotfilesDir, before)
			files, err := os.ReadDir(sourcePath)
			if err != nil {
				utils.PrintErrorCause("Error reading", sourcePath, err)
				complete = false
				continue
			}
			for _, file := range files {
				item := dot
				item.Source = filepath.Join(before, file.Name())
				item.Target = filepath.Join(dot.Target, file.Name())
				newDots = append(newDots, item)
			}
			continue
		}
		newDots = append(newDots, dot)
	}
	return newDots, complete
}

// handleDot links a single dotfile item. It returns true if a new link was
// created, and false if it already existed or linking failed, in which case
// the error is set.
func (l *linker) handleDot(item config.Item) (bool, error) {
	source := filepath.Join(l.dotfilesDir, item.Source)
	target := l.targetPath(item.Target)

	if err := l.prepareTargetSource(target, source); err != nil {
		return false, err
	}

	return l.doLink(source, target)
}

// targetPath resolves a configured target to an absolute path. Absolute
// targets are used as is, relative targets are relative to the home directory.
func (l *linker) targetPath(target string) string {
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Join(l.home, target)
}

// cleanTargets removes links that are no longer needed. For each tracked target:
//   - If it is not a symlink, it is no longer tracked.
//   - If it is a symlink that doesn't point into the dotfiles directory, it
//     was replaced by something else and is no longer tracked.
//   - If it is a symlink that isn't in the current list of dotfiles, the
//     symlink is removed and no longer tracked.
//
// It returns false if any link couldn't be removed.
func (l *linker) cleanTargets(dots []config.Item) bool {
	ok := true
	var targets []string
	for _, item := range dots {
		targets = append(targets, l.targetPath(item.Target))
	}

	l.tracked = slices.DeleteFunc(l.tracked, func(target string) bool {
		if utils.GetType(target) != utils.IsSymlink {
			return true
		} else if !pointsInto(target, l.dotfilesDir) {
			return true
		} else if !slices.Contains(targets, target) {
			utils.PrintValue("Removing link", target)
			if err := os.Remove(target); err != nil {
				utils.PrintErrorCause("Failed to remove link", target, err)
				ok = false
				return false
			}
			return true
		}
		return false
	})
	return ok
}

// prepareTargetSource prepares the target location for a symlink. It creates
// parent directories, removes an existing symlink when forcing, and moves an
// existing file or directory out of the way.
//
// An existing target is moved to the source location if the source doesn't
// exist yet. If both exist and force is enabled, the target is moved to the
// backup folder instead, which is outside of the dotfiles repository so the
// backup is never synced.
func (l *linker) prepareTargetSource(target string, source string) error {
	if err := makeDir(filepath.Dir(target)); err != nil {
		return err
	}

	switch utils.GetType(target) {
	case utils.IsFailed:
		return fmt.Errorf("couldn't access %s", target)
	case utils.IsSymlink:
		if l.force {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("couldn't remove link: %w", err)
			}
		}
	case utils.IsDirectory, utils.IsFile:
		if utils.GetType(source) == utils.NotExists {
			if err := makeDir(filepath.Dir(source)); err != nil {
				return err
			}
			if err := os.Rename(target, source); err != nil {
				return fmt.Errorf("couldn't move target to source: %w", err)
			}
			utils.PrintChange("Moving before linking", target, source)
		} else if l.force {
			backup := l.backupPath(target)
			if err := makeDir(filepath.Dir(backup)); err != nil {
				return err
			}
			if err := os.Rename(target, backup); err != nil {
				return fmt.Errorf("couldn't back up target: %w", err)
			}
			utils.PrintChange("Both source and target exist, moved target to backup", target, backup)
		} else {
			return fmt.Errorf("both source and target exist, use -f to move the target to the backup folder")
		}
	}
	return nil
}

// backupPath returns an unused backup location for a target. Backups mirror
// the target's absolute path inside the backup folder, with a number appended
// if a backup of the same target already exists.
func (l *linker) backupPath(target string) string {
	base := filepath.Join(l.backupDir, target)
	backup := base
	for count := 1; utils.GetType(backup) != utils.NotExists; count++ {
		backup = base + "." + strconv.Itoa(count)
	}
	return backup
}

// doLink creates a symbolic link from source to target. The source must exist
// and not be a symlink. The target must not exist yet, or already link to the
// source. It returns true if a new link was created, and false if it already
// existed or linking failed, in which case the error is set.
func (l *linker) doLink(source string, target string) (bool, error) {
	switch utils.GetType(source) {
	case utils.NotExists:
		return false, fmt.Errorf("source %s does not exist", source)
	case utils.IsSymlink:
		return false, fmt.Errorf("source %s is a symlink", source)
	case utils.IsFailed:
		return false, fmt.Errorf("couldn't access source %s", source)
	}

	switch utils.GetType(target) {
	case utils.NotExists:
	case utils.IsSymlink:
		if !linksTo(target, source) {
			return false, fmt.Errorf("target is a link to another location, use -f to replace it")
		}
		// Already linked. Track it in case the tracked links were lost.
		l.track(target)
		return false, nil
	default:
		return false, fmt.Errorf("target already exists")
	}

	if err := os.Symlink(source, target); err != nil {
		return false, err
	}
	utils.PrintChange("Linking", source, target)
	l.track(target)
	return true, nil
}

// track adds a target to the tracked links, if it isn't tracked yet.
func (l *linker) track(target string) {
	if !slices.Contains(l.tracked, target) {
		l.tracked = append(l.tracked, target)
	}
}

// makeDir creates a directory and its parents if it doesn't exist yet.
func makeDir(dir string) error {
	switch utils.GetType(dir) {
	case utils.IsDirectory, utils.IsSymlink:
		return nil
	case utils.IsFile:
		return fmt.Errorf("%s is not a directory", dir)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory: %w", err)
	}
	utils.PrintValue("Created directory", dir)
	return nil
}

// linkDestination returns the absolute destination of a symlink.
func linkDestination(link string) (string, error) {
	dest, err := os.Readlink(link)
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	return filepath.Clean(dest), nil
}

// pointsInto checks whether a symlink points to a path inside the given directory.
func pointsInto(link string, dir string) bool {
	dest, err := linkDestination(link)
	if err != nil {
		return false
	}
	return strings.HasPrefix(dest, filepath.Clean(dir)+string(filepath.Separator))
}

// linksTo checks whether a symlink points to the given absolute path.
func linksTo(link string, dest string) bool {
	actual, err := linkDestination(link)
	return err == nil && actual == filepath.Clean(dest)
}
