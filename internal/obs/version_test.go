package obs

import "testing"

func TestVersionIsEmptyUntilTheBuildStampsIt(t *testing.T) {
	got := Version()
	if got.Version != "" || got.Commit != "" || got.BuildTime != "" {
		t.Errorf("Version() = %+v, want empty fields", got)
	}
}

func TestStampedIsFalseForAnUnstampedBuild(t *testing.T) {
	if Version().Stamped() {
		t.Error("Stamped() = true, want false for a build without -ldflags")
	}
}

func TestStampedNeedsAllThreeFields(t *testing.T) {
	full := BuildInfo{Version: "v0.2.0", Commit: "1a2b3c4", BuildTime: "2026-10-09T10:00:00Z"}
	if !full.Stamped() {
		t.Error("Stamped() = false for a fully stamped build, want true")
	}

	partial := BuildInfo{Version: "v0.2.0"}
	if partial.Stamped() {
		t.Error("Stamped() = true without commit and build time, want false")
	}
}
