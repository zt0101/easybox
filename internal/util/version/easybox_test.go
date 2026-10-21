package version

import "testing"

func TestCompareRelease(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v3.10.2-eb.5", "3.10.2-eb.4", 1},
		{"v3.10.2-eb.4", "3.10.2-eb.4", 0},
		{"v3.10.2-eb.4", "3.10.2-eb.10", -1},
		{"v3.10.2-eb.1", "3.10.2", 1},
		{"v3.10.3", "3.10.2-eb.9", 1},
		{"v3.10.3-eb.1", "3.10.2-eb.9", 1},
		{"v3.10.2", "3.10.2", 0},
	}
	for _, tc := range cases {
		got, ok := CompareRelease(tc.a, tc.b)
		if !ok || got != tc.want {
			t.Fatalf("CompareRelease(%q, %q) = %d, %v; want %d, true", tc.a, tc.b, got, ok, tc.want)
		}
	}
	for _, bad := range []string{"v3.10.2-eb.", "v3.10.2-eb.x", "v3.10.2-eb.0", "latest"} {
		if _, ok := CompareRelease(bad, "3.10.2"); ok {
			t.Fatalf("CompareRelease(%q, ...) accepted a malformed version", bad)
		}
	}
}

func TestCompareIgnoresEasyboxSuffix(t *testing.T) {
	// Protocol gating on node panel versions compares upstream releases only.
	if cmp, ok := Compare("3.10.2-eb.5", "3.10.2"); !ok || cmp != 0 {
		t.Fatalf("Compare(3.10.2-eb.5, 3.10.2) = %d, %v; want 0, true", cmp, ok)
	}
	if cmp, ok := Compare("3.10.2-eb.5", "3.11.0"); !ok || cmp != -1 {
		t.Fatalf("Compare(3.10.2-eb.5, 3.11.0) = %d, %v; want -1, true", cmp, ok)
	}
}
