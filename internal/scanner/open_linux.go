package scanner

import (
	"golang.org/x/sys/unix"
	"strings"
)

func openDirectoryAt(rootfd int, rel string) (int, error) {
	flags := unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	fd, e := unix.Openat2(rootfd, rel, &unix.OpenHow{Flags: uint64(flags), Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS})
	if e != unix.ENOSYS && e != unix.EINVAL && e != unix.EPERM {
		return fd, e
	}
	// Older kernels: pin each component before opening the next.
	current := rootfd
	owned := false
	for _, part := range strings.Split(rel, "/") {
		next, err := unix.Openat(current, part, flags, 0)
		if owned {
			unix.Close(current)
		}
		if err != nil {
			return -1, err
		}
		current = next
		owned = true
	}
	return current, nil
}
