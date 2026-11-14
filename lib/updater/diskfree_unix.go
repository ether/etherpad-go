//go:build unix

package updater

import "golang.org/x/sys/unix"

// diskFree is the space available to this user in dir's file system.
func diskFree(dir string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
