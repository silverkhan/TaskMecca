package webui

import (
    "strings"
    "testing"

    "github.com/silverkhan/TaskMecca/goassets"
)

// Regression test: update and project-migration indicators must both retain
// full source and destination version strings on mobile and desktop.
func TestUpdateIndicatorsDoNotAbbreviateVersions(t *testing.T) {
    script, err := goassets.Template.ReadFile(embeddedRoot + "/web/app.js")
    if err != nil { t.Fatal(err) }
    source := string(script)
    for _, required := range []string{
        "class=\"update-from\"",
        "class=\"update-to\"",
        "class=\"migration-from\"",
        "class=\"migration-to\"",
        "class=\"update-versions\"",
        "class=\"migration-versions\"",
    } {
        if !strings.Contains(source, required) {
            t.Fatalf("full version markup missing: %s", required)
        }
    }
    for _, forbidden := range []string{
        "compactUpdateTargetVersion(",
        "update-version-compact",
        "update-version-full",
        "migration-version-tiny",
    } {
        if strings.Contains(source, forbidden) {
            t.Fatalf("abbreviating renderer returned: %s", forbidden)
        }
    }
    styles, err := goassets.Template.ReadFile(embeddedRoot + "/web/style.css")
    if err != nil { t.Fatal(err) }
    css := string(styles)
    for _, required := range []string{
        ".update-versions{",
        ".update-from,.update-to{white-space:normal;overflow-wrap:anywhere;",
        ".topbar .global-update-pill.available{",
        "display:inline-flex;flex-direction:column;align-items:flex-start;",
        "grid-template-rows:36px auto;",
    } {
        if !strings.Contains(css, required) {
            t.Fatalf("full version responsive style missing: %s", required)
        }
    }
    if strings.Contains(css, ".update-version-compact") || strings.Contains(css,".update-version-full") {
        t.Fatal("old version truncation CSS returned")
    }
}
