//go:build race

package mod

// raceBuild is true in a test binary built with the race detector, whose
// workers reserve address space for it that no data limit can allow.
const raceBuild = true
