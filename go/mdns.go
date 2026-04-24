package main

import (
	"github.com/grandcat/zeroconf"
)

// registerMDNS advertises the service as `airplop._http._tcp.local.` on the
// given port. Returns a shutdown func the caller should defer.
func registerMDNS(port int) (func(), error) {
	server, err := zeroconf.Register(
		"airplop",    // instance name → becomes airplop.local
		"_http._tcp", // service type
		"local.",     // domain
		port,
		[]string{"path=/"}, // TXT records
		nil,                // interfaces: nil = all
	)
	if err != nil {
		return nil, err
	}
	return server.Shutdown, nil
}
