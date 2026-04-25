package main

import (
	"errors"
	"net"
)

// lanIP returns the first non-loopback IPv4 address and the interface it's
// bound to. Returning both lets us pin mDNS to the same adapter the HTTP
// server is reachable on — otherwise zeroconf may advertise on a virtual
// adapter (WSL vEthernet, Hyper-V) that phones on the Wi-Fi can't see.
func lanIP() (string, net.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", net.Interface{}, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
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
			return ip4.String(), iface, nil
		}
	}
	return "", net.Interface{}, errors.New("no non-loopback IPv4 address found")
}
