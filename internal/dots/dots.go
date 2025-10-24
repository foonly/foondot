package dots

import (
	"os"
	"path"
	"slices"
	"strconv"

	"foonly.dev/foondot/internal/config"
	"foonly.dev/foondot/internal/utils"
	"github.com/adrg/xdg"
)

/**
 * Filters a list of dotfile items based on hostname. If a dotfile item has a
 * hostname defined, it is included in the filtered list only if the current
 * hostname is present in the dotfile's hostname list. If a dotfile item does
 * not have a hostname defined, it is always included in the filtered list.
 *
 * @param dots A slice of Item structs representing the dotfile items to filter.
 * @return A new slice of Item structs containing only the dotfile items that
 *         match the hostname criteria.
 */
func FilterDots(dots []config.Item) []config.Item {
	newDots := []config.Item{}
	for _, dot := range dots {
		if len(dot.Hostname) == 0 || slices.Contains(dot.Hostname, config.Hostname) {
			newDots = append(newDots, dot)
		}
	}
	return newDots
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
func HandleDot(item config.Item, dotfiles string, force bool) bool {

	source := path.Join(xdg.Home, dotfiles, item.Source)
	target := path.Join(xdg.Home, item.Target)

	prepareTargetSource(target, source, force)

	return doLink(source, target)
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
