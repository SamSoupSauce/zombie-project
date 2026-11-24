//go:build !windows

package camera

// trapMouseToWindow is a no-op on non-Windows platforms.
func trapMouseToWindow() {}

// releaseMouseFromWindow is a no-op on non-Windows platforms.
func releaseMouseFromWindow() {}

// hidePlatformCursor is a no-op on non-Windows platforms.
func hidePlatformCursor() {}

// showPlatformCursor is a no-op on non-Windows platforms.
func showPlatformCursor() {}
