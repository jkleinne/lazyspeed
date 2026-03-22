//go:build windows

package diag

import "syscall"

// setTTL sets the IP TTL on a socket file descriptor (Windows).
func setTTL(fd uintptr, ttl int) error {
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
}
