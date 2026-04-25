package main

import (
	"errors"
	"fmt"
	"net"
)

// "Connect" a UDP socket to a public IP — no packet sent, but the kernel
// fills in the local end with the source IP it would route through. Then
// we map that IP back to its interface so mDNS can pin to it.
func lanIP() (string, net.Interface, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return "", net.Interface{}, fmt.Errorf("udp route probe: %w", err) // %w wraps err for errors.Is/As
	}
	defer conn.Close()
	local := conn.LocalAddr().(*net.UDPAddr) // x.(T) type assertion (panics on miss)

	ifaces, err := net.Interfaces()
	if err != nil {
		return "", net.Interface{}, err
	}
	for _, iface := range ifaces { // range over slice yields (index, value); _ ignores index
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipnet, ok := a.(*net.IPNet) // comma-ok form: ok=false instead of panic
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
