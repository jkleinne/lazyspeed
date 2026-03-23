//go:build !windows

package diag

import "syscall"

// setTTL sets the IP TTL on a socket file descriptor (Unix).
func setTTL(fd uintptr, ttl int) error {
	return syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
}
