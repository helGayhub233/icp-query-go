package netutil

import (
	"net"
	"net/url"
	"strings"
)

// IsPublicIPv6 checks if an IPv6 address is a public (global unicast) address.
func IsPublicIPv6(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	return ip.To4() == nil && ip.IsGlobalUnicast()
}

// GetLocalIPv6Addresses returns all public IPv6 addresses from system interfaces.
func GetLocalIPv6Addresses() []string {
	seen := make(map[string]bool)
	var addresses []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP
			if ip.To4() != nil {
				continue
			}
			if !ip.IsGlobalUnicast() {
				continue
			}
			ipStr := ip.String()
			if !seen[ipStr] {
				seen[ipStr] = true
				addresses = append(addresses, ipStr)
			}
		}
	}

	return addresses
}

// IsValidURL checks if a URL string is a valid HTTP/HTTPS URL.
func IsValidURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")
}
