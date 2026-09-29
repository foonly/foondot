package dots

import (
	"errors"
	"fmt"
	"os"
	"path"
	"slices"
	"strconv"
	"strings"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

/**
 * Orchestrates the linking process for dotfiles. It reads the current state,
 * filters dotfiles based on the current hostname, handles the linking of
 * each dotfile, cleans up any obsolete links, and persists the new state.
 *
 * @param cfg The configuration object containing dotfile definitions and settings.
 * @param force Whether to force the linking process, overriding existing files if necessary.
 * @return error Set if any dotfile couldn't be linked or any old link couldn't be removed.
 */
func Link(cfg config.Config, force bool) error {
	config.ReadDotsData()

	dotFiles, complete := filterDots(cfg.Dotfiles, cfg.Dots)

	var failures []string
	if complete {
		if !cleanTargets(path.Join(xdg.Home, cfg.Dotfiles), dotFiles) {
			failures = append(failures, "some old links couldn't be removed")
		}
	} else {
		utils.PrintError("Some sources could not be read, skipping removal of old links")
		failures = append(failures, "some sources couldn't be read")
	}

	numberLinked := 0
	numberFailed := 0
	for _, element := range dotFiles {
		linked, err := handleDot(element, cfg.Dotfiles, force)
		if err != nil {
			utils.PrintError("Couldn't link", targetPath(element.Target), err.Error())
			numberFailed++
		} else if linked {
			numberLinked++
		}
	}
	if numberFailed > 0 {
		failures = append(failures, fmt.Sprintf("%d of %d dotfiles couldn't be linked", numberFailed, len(dotFiles)))
	}

	config.WriteDotsData()

	if force {
		fmt.Fprintf(os.Stdout, "Force mode enabled\n")
	}
	if numberLinked == 0 {
		fmt.Fprintf(os.Stdout, "No new dotfiles linked.\n")
	} else if numberLinked == len(dotFiles) {
		fmt.Fprintf(os.Stdout, "All %d dotfiles linked.\n", len(dotFiles))
	} else {
		fmt.Fprintf(os.Stdout, "%d of %d dotfiles linked.\n", numberLinked, len(dotFiles))
	}

	if len(failures) > 0 {
		return errors.New(strings.Join(failures, ", "))
	}
	return nil
}

/**
 * Filters a list of dotfile items based on hostname. If a dotfile item has a
 * hostname defined, it is included in the filtered list only if the current
 * hostname is present in the dotfile's hostname list. If a dotfile item does
 * not have a hostname defined, it is always included in the filtered list.
 *
 * @param dots A slice of Item structs representing the dotfile items to filter.
 * @return A new slice of Item structs containing only the dotfile items that
 *         match the hostname criteria, and false if a wildcard source could
 *         not be read (the returned list is then incomplete).
 */
func filterDots(dotfileFolder string, dots []config.Item) ([]config.Item, bool) {
	newDots := []config.Item{}
	complete := true
	for _, dot := range dots {
		if len(dot.Hostname) > 0 && !slices.Contains(dot.Hostname, config.Hostname) {
			continue
		}
		// If the source ends with /* expand it to include all files in the directory
		if before, ok := strings.CutSuffix(dot.Source, "/*"); ok {
			sourcePath := path.Join(xdg.Home, dotfileFolder, before)
			// Loop all files and folders in newSource and add them as separate dotfile items
			files, err := os.ReadDir(sourcePath)
			if err != nil {
				utils.PrintError("Error reading", sourcePath, err.Error())
				complete = false
				continue
			}
			for _, file := range files {
				item := dot
				item.Source = path.Join(before, file.Name())
				item.Target = path.Join(dot.Target, file.Name())
				newDots = append(newDots, item)
			}
			continue
		}
		newDots = append(newDots, dot)
	}
	return newDots, complete
}

/**
* Handles a single dotfile item, determining source and target paths,
* preparing the target location, and creating the symlink.
*
* @param item The dotfile item to handle.
* @param dotfiles The base directory for dotfiles.
* @param force Whether to force relinking and move existing files.
* @return True if a new link was created, false if it failed or already existed.
* @return error Set if the dotfile couldn't be linked.
 */
func handleDot(item config.Item, dotfiles string, force bool) (bool, error) {
	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := targetPath(item.Target)

	if err := prepareTargetSource(target, source, force); err != nil {
		return false, err
	}

	return doLink(source, target)
}

/**
 * Resolves a configured target to an absolute path. Absolute targets are used
 * as is, relative targets are relative to the home directory.
 *
 * @param target The target as given in the configuration.
 * @return The absolute path to the target.
 */
func targetPath(target string) string {
	if path.IsAbs(target) {
		return path.Clean(target)
	}
	return path.Join(xdg.Home, target)
}

/**
 * Cleans up target symlinks that are no longer valid or needed.
 * Iterates through the list of tracked dotfile targets (config.DotsData),
 * and for each target:
 *   - If the target is not a symlink, it is removed from the tracking array.
 *   - If the target is a symlink that no longer points into the dotfiles
 *     directory, it was replaced by something else and is only removed from
 *     the tracking array.
 *   - If the target is a symlink but does not exist in the current list of
 *     dotfile targets, the symlink is removed from the filesystem and from
 *     the tracking array.
 *
 * @param dotfilesDir The absolute path to the dotfiles directory.
 * @param dots A slice of Item structs representing the current dotfile items.
 * @return False if any link couldn't be removed.
 */
func cleanTargets(dotfilesDir string, dots []config.Item) bool {
	ok := true
	var targets []string
	// Create a list of targets from defined dots.
	for _, item := range dots {
		targets = append(targets, targetPath(item.Target))
	}

	config.DotsData = slices.DeleteFunc(config.DotsData, func(target string) bool {
		if utils.GetType(target) != utils.IsSymlink {
			// Target is not a symlink, remove from list.
			return true
		} else if !pointsInto(target, dotfilesDir) {
			// Target is a symlink not created by us, stop tracking it.
			return true
		} else if !slices.Contains(targets, target) {
			// Target is a symlink and doesn't exist in the list of targets.
			utils.PrintMessage("Removing link", target)
			err := os.Remove(target)
			if err != nil {
				utils.PrintError("Failed to remove link", target, err.Error())
				ok = false
				return false
			}
			return true
		}
		return false
	})
	return ok
}

/**
 * Checks whether a symlink points to a path inside the given directory.
 *
 * @param link The path to the symlink.
 * @param dir The absolute path to the directory.
 * @return True if the symlink destination is inside dir, false otherwise.
 */
func pointsInto(link string, dir string) bool {
	dest, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !path.IsAbs(dest) {
		dest = path.Join(path.Dir(link), dest)
	}
	return strings.HasPrefix(path.Clean(dest), path.Clean(dir)+"/")
}

/**
 * Prepares the target location for a symlink. This includes creating parent
 * directories, removing existing symlinks (if force is enabled), and moving
 * existing files or directories out of the way to avoid conflicts.
 *
 * An existing target is moved to the source location if the source doesn't
 * exist yet. If both exist and force is enabled, the target is moved to the
 * backup folder instead, which is outside of the dotfiles repository so the
 * backup is never synced.
 *
 * @param target The path to the target location for the symlink.
 * @param source The path to the source file or directory that will be linked.
 * @param force Whether to force relinking, moving existing files if necessary.
 * @return error Set if the target location couldn't be prepared.
 */
func prepareTargetSource(target string, source string, force bool) error {
	if err := makeDir(path.Dir(target)); err != nil {
		return err
	}

	targetType := utils.GetType(target)
	switch targetType {
	case utils.IsFailed:
		return fmt.Errorf("couldn't access %s", target)
	case utils.IsSymlink:
		if force {
			if err := os.Remove(target); err != nil {
				return fmt.Errorf("couldn't remove link: %w", err)
			}
		}
	case utils.IsDirectory, utils.IsFile:
		sourceType := utils.GetType(source)
		if sourceType == utils.NotExists {
			if err := makeDir(path.Dir(source)); err != nil {
				return err
			}
			if err := os.Rename(target, source); err != nil {
				return fmt.Errorf("couldn't move target to source: %w", err)
			}
			utils.PrintMessage("Moving before linking", target, source)
		} else if force {
			backup := backupPath(target)
			if err := makeDir(path.Dir(backup)); err != nil {
				return err
			}
			if err := os.Rename(target, backup); err != nil {
				return fmt.Errorf("couldn't back up target: %w", err)
			}
			utils.PrintMessage("Both source and target exist, moved target to backup", target, backup)
		} else {
			return fmt.Errorf("both source and target exist, use -f to move the target to the backup folder")
		}
	}
	return nil
}

/**
 * Creates a directory and its parents if it doesn't exist yet.
 *
 * @param dir The path to the directory.
 * @return error Set if the directory couldn't be created.
 */
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
	utils.PrintMessage("Created directory", dir)
	return nil
}

/**
 * Returns an unused backup location for a target. Backups mirror the target's
 * absolute path inside the backup folder, with a number appended if a backup
 * of the same target already exists.
 *
 * @param target The absolute path to the target.
 * @return The path to back up the target to.
 */
func backupPath(target string) string {
	base := path.Join(config.BackupFolder(), target)
	backup := base
	for count := 1; utils.GetType(backup) != utils.NotExists; count++ {
		backup = base + "." + strconv.Itoa(count)
	}
	return backup
}

/**
 * Creates a symbolic link from source to target. Checks if source exists and
 * is not a symlink. Checks if the target does not exist yet, or already links
 * to the source.
 *
 * @param source The path to the source file or directory.
 * @param target The path to the target location for the symlink.
 * @return True if a new link was created, false if it failed or already existed.
 * @return error Set if the link couldn't be created.
 */
func doLink(source string, target string) (bool, error) {
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
		// Already linked. Track it in case the dots data was lost.
		track(target)
		return false, nil
	default:
		return false, fmt.Errorf("target already exists")
	}

	if err := os.Symlink(source, target); err != nil {
		return false, err
	}
	utils.PrintMessage("Linking", source, target)
	track(target)
	return true, nil
}

/**
 * Adds a target to the tracked links, if it isn't tracked yet.
 *
 * @param target The path to the link.
 */
func track(target string) {
	if !slices.Contains(config.DotsData, target) {
		config.DotsData = append(config.DotsData, target)
	}
}

/**
 * Checks whether a symlink points to the given path.
 *
 * @param link The path to the symlink.
 * @param dest The expected absolute destination.
 * @return True if the symlink points to dest, false otherwise.
 */
func linksTo(link string, dest string) bool {
	actual, err := os.Readlink(link)
	if err != nil {
		return false
	}
	if !path.IsAbs(actual) {
		actual = path.Join(path.Dir(link), actual)
	}
	return path.Clean(actual) == path.Clean(dest)
}
