package obs

// Overwritten at build time via -ldflags from v0.2.0 on. They already exist here
// so that release is a build change and not a code change.
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

type BuildInfo struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

func Version() BuildInfo {
	return BuildInfo{Version: version, Commit: commit, BuildTime: buildTime}
}
