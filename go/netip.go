package main

import (
	"errors"
	"fmt"
	"net"
)

// lanIP returns the LAN-facing IPv4 address and the interface that owns it.
//
// We "connect" a UDP socket to a public IP — no packet is actually sent, but
// the kernel populates the local end of the socket with the source IP it
// would route through, i.e. the interface bound to the default gateway.
// In WSL2 mirrored mode this is eth0 with the host's LAN IP. We then map the
// IP back to its interface so mDNS can pin its broadcasts to the same one.
func lanIP() (string, net.Interface, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return "", net.Interface{}, fmt.Errorf("udp route probe: %w", err)
	}
	defer conn.Close()
	local := conn.LocalAddr().(*net.UDPAddr)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", net.Interface{}, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			if ipnet.IP.Equal(local.IP) {
				return local.IP.String(), iface, nil
			}
		}
	}
	return "", net.Interface{}, errors.New("no interface owns " + local.IP.String())
}
