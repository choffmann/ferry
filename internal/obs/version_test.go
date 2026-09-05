package obs

import "testing"

func TestVersionDefaultsMarkAnUnstampedBuild(t *testing.T) {
	got := Version()
	if got.Version != "dev" || got.Commit != "none" || got.BuildTime != "unknown" {
		t.Errorf("Version() = %+v, want the dev placeholders", got)
	}
}
