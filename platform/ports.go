package platform

import "fmt"

// browserBlockedPorts are ports Chromium/Firefox refuse to load
// (ERR_UNSAFE_PORT). Hosting a landing page on these makes the project look
// "running" while the browser shows an error.
var browserBlockedPorts = map[uint]bool{
	6000:  true, // X11
	6665:  true,
	6666:  true,
	6667:  true,
	6668:  true,
	6669:  true,
	10080: true,
}

// ValidatePublicPort rejects ports browsers will not connect to.
func ValidatePublicPort(port uint) error {
	if port == 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if browserBlockedPorts[port] {
		return fmt.Errorf("port %d is blocked by browsers (ERR_UNSAFE_PORT); choose 8080 or another safe port", port)
	}
	return nil
}
