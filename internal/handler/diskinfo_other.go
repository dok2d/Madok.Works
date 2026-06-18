//go:build !linux

package handler

// diskUsage is unsupported off Linux; the runtime image is Linux.
func diskUsage(path string) (free, total uint64) { return 0, 0 }
