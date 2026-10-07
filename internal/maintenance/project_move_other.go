//go:build !darwin

package maintenance

import (
	"fmt"
	"os"
)

func moveProjectFolder(source, destination string, expected os.FileInfo) error {
	return fmt.Errorf("recoverable folder cleanup is supported only by macOS Trash; folder preserved")
}
