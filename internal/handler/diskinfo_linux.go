//go:build linux

package handler

import "syscall"

// diskUsage returns the available and total bytes of the filesystem holding
// path. Returns zeros if the path can't be stat'd.
func diskUsage(path string) (free, total uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, 0
	}
	bs := uint64(st.Bsize)
	return st.Bavail * bs, st.Blocks * bs
}
