package vcs

import (
	"fmt"
	"runtime/debug"
)

//Return the version of the application (based on the git commit hash) and whether it has been
//modified since last commit.
func Version() string {
    var revision string
    var modified bool
    bi, ok := debug.ReadBuildInfo()
    if ok {
        for _, s := range bi.Settings {
            switch s.Key {
            case "vcs.revision":
                revision = s.Value
            case "vcs.modified":
                if s.Value == "true" {
                    modified = true
                }
            }
        }
    }
    if modified {
        return fmt.Sprintf("%s-dirty", revision)
    }
    return revision
 }
 