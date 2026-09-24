//go:build !linux

package audio

// closeInheritedFDsOnExec is a no-op off Linux.
func closeInheritedFDsOnExec() {}
