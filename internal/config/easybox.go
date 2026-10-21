package config

import "strings"

// easyboxRelease is the "eb.N" part of an EasyBox release tag (v<upstream>-eb.N),
// injected via `-ldflags -X` by .github/workflows/release.yml. It stays empty for
// local builds, dev builds and the upstream-shaped Docker build.
var easyboxRelease string

// EasyboxRelease returns "eb.N" for an EasyBox release build, or "".
func EasyboxRelease() string {
	return strings.TrimSpace(easyboxRelease)
}

// GetReleaseVersion returns the release this binary was built from: "3.10.2-eb.4"
// for an EasyBox release build, the plain upstream version otherwise.
func GetReleaseVersion() string {
	if eb := EasyboxRelease(); eb != "" {
		return GetBaseVersion() + "-" + eb
	}
	return GetBaseVersion()
}
