// Package version holds the build version string.
//
// It is its own package rather than a variable in cmd/mop because internal
// packages cannot import the main package.
package version

// Version is overridden at build time with -ldflags "-X ...version.Version=x.y.z".
var Version = "dev"
