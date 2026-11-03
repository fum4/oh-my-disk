package scanner

import "golang.org/x/sys/unix"

func openDirectoryAt(rootfd int, rel string) (int, error) {
	return unix.Openat(rootfd, rel, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW_ANY, 0)
}
