package main

import (
	"net"

	"github.com/grandcat/zeroconf"
)

// registerMDNS advertises the service as `airplop._http._tcp.local.` on the
// given port, pinning both the advertised IP and the network interface
// zeroconf sends on. Returns a shutdown func the caller should defer.
func registerMDNS(ip string, iface net.Interface, port int) (func(), error) {
	server, err := zeroconf.RegisterProxy(
		"airplop",                // instance name
		"_http._tcp",             // service type
		"local.",                 // domain
		port,
		"airplop",                // host → airplop.local
		[]string{ip},             // advertise this IP only
		[]string{"path=/"},       // TXT records
		[]net.Interface{iface},   // only send on this adapter
	)
	if err != nil {
		return nil, err
	}
	return server.Shutdown, nil
}
