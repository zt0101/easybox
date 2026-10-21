package version

import (
	"strconv"
	"strings"
)

// EasyBox release tags are v<upstream>-eb.N (e.g. v3.10.2-eb.4).

// splitEasybox splits "3.10.2-eb.4" into "3.10.2" and 4. A version without the
// suffix returns n = 0; a malformed suffix returns ok = false.
func splitEasybox(v string) (base string, n int, ok bool) {
	base, suffix, found := strings.Cut(strings.TrimSpace(v), "-eb.")
	if !found {
		return base, 0, true
	}
	n, err := strconv.Atoi(suffix)
	if err != nil || n < 1 {
		return v, 0, false
	}
	return base, n, true
}

// CompareRelease is Compare for EasyBox releases: it orders the upstream version
// first, then the eb number (a version without one counts as eb.0).
func CompareRelease(a, b string) (int, bool) {
	aBase, aN, okA := splitEasybox(a)
	bBase, bN, okB := splitEasybox(b)
	if !okA || !okB {
		return 0, false
	}
	cmp, ok := Compare(aBase, bBase)
	if !ok || cmp != 0 {
		return cmp, ok
	}
	switch {
	case aN > bN:
		return 1, true
	case aN < bN:
		return -1, true
	}
	return 0, true
}
