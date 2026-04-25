package main

import (
	"net"

	"github.com/grandcat/zeroconf" // third-party import: URL-style path resolved via go.mod
)

// pin to one IP and one interface so zeroconf doesn't auto-pick the wrong one
func registerMDNS(ip string, iface net.Interface, port int) (func(), error) {
	server, err := zeroconf.RegisterProxy(
		"airplop",              // instance name
		"_http._tcp",           // service type
		"local.",               // domain
		port,
		"airplop",              // host → airplop.local
		[]string{ip},           // []T{...} slice composite literal
		[]string{"path=/"},     // TXT records
		[]net.Interface{iface},
	)
	if err != nil {
		return nil, err
	}
	return server.Shutdown, nil // method value: bound, callable later
}
