package dots

import (
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
 */
func Link(cfg config.Config, force bool) {
	config.ReadDotsData()

	dotFiles, complete := filterDots(cfg.Dotfiles, cfg.Dots)

	if complete {
		cleanTargets(path.Join(xdg.Home, cfg.Dotfiles), dotFiles)
	} else {
		utils.PrintError("Some sources could not be read, skipping removal of old links")
	}

	numberLinked := 0
	for _, element := range dotFiles {
		if handleDot(element, cfg.Dotfiles, force) {
			numberLinked++
		}
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
* @return True if the link was successfully created, false otherwise.
 */
func handleDot(item config.Item, dotfiles string, force bool) bool {

	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := path.Join(xdg.Home, item.Target)

	prepareTargetSource(target, source, force)

	return doLink(source, target)
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
 */
func cleanTargets(dotfilesDir string, dots []config.Item) {
	var targets []string
	// Create a list of targets from defined dots.
	for _, item := range dots {
		targets = append(targets, path.Join(xdg.Home, item.Target))
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
				return false
			}
			return true
		}
		return false
	})

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
 * @param target The path to the target location for the symlink.
 * @param source The path to the source file or directory that will be linked.
 * @param force Whether to force relinking, moving existing files if necessary.
 */
func prepareTargetSource(target string, source string, force bool) {
	targetDir := path.Dir(target)
	if utils.GetType(targetDir) == utils.NotExists {
		err := os.MkdirAll(targetDir, os.ModePerm)
		if err == nil {
			// No error means directory was created.
			utils.PrintMessage("Created directory", targetDir)
		}
	}

	targetType := utils.GetType(target)

	if targetType == utils.IsSymlink && force {
		// Remove target if it's a symlink.
		os.Remove(target)
	}
	if targetType == utils.IsDirectory || targetType == utils.IsFile {
		// Target is not a symlink.
		isDirFile := "file"
		if targetType == utils.IsDirectory {
			isDirFile = "directory"
		}
		utils.PrintError("Target is a "+isDirFile, target)
		sourceType := utils.GetType(source)

		if sourceType == utils.NotExists {
			sourceDir := path.Dir(source)
			if utils.GetType(sourceDir) == utils.NotExists {
				err := os.MkdirAll(sourceDir, os.ModePerm)
				if err == nil {
					// No error means directory was created.
					utils.PrintMessage("Created directory", sourceDir)
				} else {
					utils.PrintError("Couldn't create directory", sourceDir)
				}
			}

			moveErr := os.Rename(target, source)
			if moveErr == nil {
				utils.PrintMessage("Moving before linking", target, source)
			}
		} else if force {
			utils.PrintMessage("force", source)
			sourceConflict := source + ".conflict"
			count := 0
			for {
				// Find an available filename
				conflictType := utils.GetType(sourceConflict)
				if conflictType == utils.NotExists {
					break
				}
				count++
				sourceConflict = source + ".conflict." + strconv.Itoa(count)
			}

			err := os.Rename(target, sourceConflict)
			if err == nil {
				utils.PrintMessage("Both source and target exist, forcing move out of the way", target, sourceConflict)
			} else {
				utils.PrintError("Couldn't backup target, skipping", target)
			}
		} else {
			utils.PrintError("Both source and target exist. Skipping", source, "Use -f to override.")
		}
	}
}

/**
 * Creates a symbolic link from source to target. Checks if source exists and
 * is not a symlink. Checks if the target does not exist and the source is
 * either a directory or a file.
 *
 * @param source The path to the source file or directory.
 * @param target The path to the target location for the symlink.
 * @return True if the link was successfully created, false otherwise.
 */
func doLink(source string, target string) bool {
	sourceType := utils.GetType(source)
	targetType := utils.GetType(target)

	if sourceType == utils.NotExists {
		utils.PrintError("Source does not exist", source)
		return false
	}
	if sourceType == utils.IsSymlink {
		utils.PrintError("Source is a symlink", source)
		return false
	}

	if targetType == utils.NotExists && (sourceType == utils.IsDirectory || sourceType == utils.IsFile) {
		err := os.Symlink(source, target)
		utils.PrintMessage("Linking", source, target)
		if err == nil {
			if !slices.Contains(config.DotsData, target) {
				config.DotsData = append(config.DotsData, target)
			}
		} else {
			utils.PrintError("Error linking", target)
		}
		return err == nil
	}
	return false
}
