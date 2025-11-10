package platform

import "runtime"

// OS represents the operating system
type OS string

const (
	macOS   OS = "darwin"
	Linux   OS = "linux"
	Windows OS = "windows"
)

// Current returns the current operating system
func Current() OS {
	return OS(runtime.GOOS)
}

// IsMac returns true if running on macOS
func IsMac() bool {
	return runtime.GOOS == "darwin"
}

// IsLinux returns true if running on Linux
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsWindows returns true if running on Windows
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
