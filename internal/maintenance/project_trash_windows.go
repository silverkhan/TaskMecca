//go:build windows

package maintenance

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

//go:embed project_trash_windows.ps1
var windowsRecycleScript string

func encodedPowerShell(script string) string {
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		data[i*2] = byte(unit)
		data[i*2+1] = byte(unit >> 8)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func recycleStagedFolder(source string, expected os.FileInfo) (string, error) {
	if err := validateTrashPlatform(source); err != nil {
		return "", err
	}
	parents, err := lockCleanupParents(filepath.Dir(source))
	if err != nil {
		return "", err
	}
	defer closeCleanupHandles(parents)
	name, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return "", err
	}
	f := os.NewFile(uintptr(handle), source)
	defer f.Close()
	current, err := f.Stat()
	var id windows.ByHandleFileInformation
	if err != nil || !os.SameFile(expected, current) || windows.GetFileInformationByHandle(windows.Handle(f.Fd()), &id) != nil || id.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return "", fmt.Errorf("staging identity changed; native recycle refused")
	}
	input, err := json.Marshal(map[string]any{"path": source, "volume": id.VolumeSerialNumber, "high": id.FileIndexHigh, "low": id.FileIndexLow})
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(windowsRecycleScript))
	cmd.Stdin = strings.NewReader(string(input))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Windows native Recycle Bin refused (no permanent-delete fallback): %s: %w", strings.TrimSpace(string(output)), err)
	}
	var result struct {
		Location string `json:"location"`
		Success  bool   `json:"success"`
	}
	if err := json.Unmarshal(output, &result); err != nil || !result.Success || result.Location == "" {
		return "", fmt.Errorf("Windows native recycle outcome unconfirmed: %s", strings.TrimSpace(string(output)))
	}
	return result.Location, nil
}
