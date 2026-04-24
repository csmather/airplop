package main

import (
	"errors"
	"net"
)

// lanIP returns the first non-loopback IPv4 address bound to any interface.
// On native Windows this is enough — unlike the WSL2 Python version, there's
// no virtual 172.x network or PowerShell fallback to worry about.
func lanIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil {
			continue
		}
		return ip4.String(), nil
	}
	return "", errors.New("no non-loopback IPv4 address found")
}
