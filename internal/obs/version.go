package obs

// Set at build time via -ldflags. Empty is the honest answer for a binary built
// without them: nobody can tell which commit is running.
var (
	version   string
	commit    string
	buildTime string
)

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Version() BuildInfo {
	return BuildInfo{Version: version, Commit: commit, BuildTime: buildTime}
}

// Stamped reports whether the build passed all three values in.
func (b BuildInfo) Stamped() bool {
	return b.Version != "" && b.Commit != "" && b.BuildTime != ""
}
