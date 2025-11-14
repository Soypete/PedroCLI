package platform

import (
	"runtime"
	"testing"
)

func TestCurrent(t *testing.T) {
	current := Current()
	expected := OS(runtime.GOOS)

	if current != expected {
		t.Errorf("Current() = %v, want %v", current, expected)
	}
}

func TestIsMac(t *testing.T) {
	result := IsMac()
	expected := runtime.GOOS == "darwin"

	if result != expected {
		t.Errorf("IsMac() = %v, want %v", result, expected)
	}
}

func TestIsLinux(t *testing.T) {
	result := IsLinux()
	expected := runtime.GOOS == "linux"

	if result != expected {
		t.Errorf("IsLinux() = %v, want %v", result, expected)
	}
}

func TestIsWindows(t *testing.T) {
	result := IsWindows()
	expected := runtime.GOOS == "windows"

	if result != expected {
		t.Errorf("IsWindows() = %v, want %v", result, expected)
	}
}
