//go:build !darwin && !windows

package maintenance

import (
	"fmt"
	"os"
)

func moveProjectFolder(source, destination string, expected os.FileInfo) error {
	return fmt.Errorf("recoverable folder cleanup is supported only by macOS Trash; folder preserved")
}

func validateTrashPlatform(source string) error {
	return fmt.Errorf("recoverable folder cleanup is not supported on this operating system; folder preserved")
}
func recycleStagedFolder(source string, expected os.FileInfo) (string, error) {
	return "", validateTrashPlatform(source)
}
