package panel

import "testing"

func TestIsNewerVersionEasyboxTags(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v3.10.2-eb.4", "3.10.2-eb.4", false},
		{"v3.10.2-eb.5", "3.10.2-eb.4", true},
		{"v3.10.2-eb.4", "3.10.2-eb.5", false},
		// eb.1–eb.4 binaries were not stamped and report the bare upstream version.
		{"v3.10.2-eb.5", "3.10.2", true},
	}
	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}
