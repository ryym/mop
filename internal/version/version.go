// Package version holds the build version string.
//
// It is its own package (not a variable in cmd/mop) because both the state
// file and the daemon's /api/status need it, and internal packages cannot
// import the main package.
package version

// Version is overridden at build time with -ldflags "-X ...version.Version=x.y.z".
var Version = "dev"
