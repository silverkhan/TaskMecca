package backlog

import (
    "regexp"
    "strings"
)

var sourceLinkPattern = regexp.MustCompile(`^\[([^\]]+)\]\((https?://[^)]+)\)$`)

func sourceFromFields(fields map[string]string) map[string]string {
    raw:=strings.TrimSpace(fields["출처"])
    if raw=="" || raw=="-" { return map[string]string{} }
    out:=map[string]string{"raw":raw}
    label:=raw
    if match:=sourceLinkPattern.FindStringSubmatch(raw); match!=nil {
        label=strings.TrimSpace(match[1])
        out["url"]=strings.TrimSpace(match[2])
    }
    label=strings.ReplaceAll(label,"`","")
    out["label"]=label
    if parts:=strings.SplitN(label," · ",2); len(parts)==2 {
        if provider:=strings.TrimSpace(parts[0]); provider!="" { out["provider"]=provider }
        if reference:=strings.TrimSpace(parts[1]); reference!="" { out["reference"]=reference }
    }
    return out
}
