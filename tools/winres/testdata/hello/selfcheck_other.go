//go:build !windows

package main

// selfCheck has nothing to verify on systems without Windows resources.
func selfCheck() error { return nil }
