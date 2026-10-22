package config

import "testing"

func TestGetReleaseVersionAppendsEasyboxRelease(t *testing.T) {
	saved, savedCommit := easyboxRelease, buildCommit
	t.Cleanup(func() { easyboxRelease, buildCommit = saved, savedCommit })

	buildCommit = ""
	easyboxRelease = ""
	if got := GetPanelVersion(); got != GetBaseVersion() {
		t.Fatalf("unstamped build: GetPanelVersion() = %q, want %q", got, GetBaseVersion())
	}

	easyboxRelease = "eb.5"
	want := GetBaseVersion() + "-eb.5"
	if got := GetReleaseVersion(); got != want {
		t.Fatalf("GetReleaseVersion() = %q, want %q", got, want)
	}
	if got := GetPanelVersion(); got != want {
		t.Fatalf("GetPanelVersion() = %q, want %q", got, want)
	}

	// A dev build keeps its dev+<sha> identity.
	buildCommit = "0123456789"
	if got := GetPanelVersion(); got != "dev+01234567" {
		t.Fatalf("dev build: GetPanelVersion() = %q, want dev+01234567", got)
	}
}
