//go:build darwin

package app

import (
	"golang.org/x/sys/unix"
)

// freeDisk is the free space on the filesystem holding dir, in bytes.
func freeDisk(dir string) (int64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return int64(st.Bavail) * int64(st.Bsize), nil
}
