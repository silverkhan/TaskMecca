//go:build windows

package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func restoreNativeTrashFixture(location, stage string) error {
	const script = `$ErrorActionPreference='Stop'
$record=[Console]::In.ReadToEnd() | ConvertFrom-Json
$shell=New-Object -ComObject Shell.Application
$bin=$shell.Namespace(10)
$matches=@($bin.Items() | Where-Object {
  $deletedFrom=$_.ExtendedProperty('System.Recycle.DeletedFrom')
  ($_.Path -eq $record.location) -or (($deletedFrom -eq [IO.Path]::GetDirectoryName($record.stage)) -and ($_.Name -eq [IO.Path]::GetFileName($record.stage)))
})
if($matches.Count -ne 1){throw "Unique recycle fixture not found ($($matches.Count))"}
$matches[0].InvokeVerb('undelete')
for($i=0;$i -lt 100;$i++){if(Test-Path -LiteralPath $record.stage){exit 0};Start-Sleep -Milliseconds 100}
throw 'Recycle fixture native restore timed out'`
	input, _ := json.Marshal(map[string]string{"location": location, "stage": stage})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-EncodedCommand", encodedPowerShell(script))
	cmd.Stdin = strings.NewReader(string(input))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Windows native restore: %s: %w", output, err)
	}
	if _, err := os.Lstat(stage); err != nil {
		return err
	}
	return nil
}
