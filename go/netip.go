package main

import (
	"fmt"
	"net"
)

// "Connect" a UDP socket to a public IP — no packet sent, but the kernel
// fills in the local end with the source IP it would route through.
func lanIP() (string, error) {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("udp route probe: %w", err) // %w wraps err for errors.Is/As
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil // x.(T) type assertion (panics on miss)
}
