// Package client contains types and logic dealing with AdGuard Home's DNS
// clients.
//
// TODO(a.garipov): Expand.
package client

import (
	"encoding"
	"fmt"

	"github.com/AdguardTeam/AdGuardHome/internal/whois"
)

// Source represents the source from which the information about the client has
// been obtained.
type Source uint8

// Clients information sources.  The order determines the priority.
const (
	SourceWHOIS Source = iota + 1
	SourceARP
	SourceRDNS
	SourceDHCP
	SourceHostsFile
	SourcePersistent
)

// type check
var _ fmt.Stringer = Source(0)

// String returns a human-readable name of cs.
func (cs Source) String() (s string) {
	switch cs {
	case SourceWHOIS:
		return "WHOIS"
	case SourceARP:
		return "ARP"
	case SourceRDNS:
		return "rDNS"
	case SourceDHCP:
		return "DHCP"
	case SourceHostsFile:
		return "etc/hosts"
	default:
		return ""
	}
}

// type check
var _ encoding.TextMarshaler = Source(0)

// MarshalText implements encoding.TextMarshaler for the Source.
func (cs Source) MarshalText() (text []byte, err error) {
	return []byte(cs.String()), nil
}

// Runtime is a client information from different sources.
type Runtime struct {
	// whois is the filtered WHOIS information of a client.
	whois *whois.Info

	// arp is the ARP information of a client.  nil indicates that there is no
	// information from the source.  Empty non-nil slice indicates that the data
	// from the source is present, but empty.
	arp []string

	// rdns is the RDNS information of a client.  nil indicates that there is no
	// information from the source.  Empty non-nil slice indicates that the data
	// from the source is present, but empty.
	rdns []string

	// dhcp is the DHCP information of a client.  nil indicates that there is no
	// information from the source.  Empty non-nil slice indicates that the data
	// from the source is present, but empty.
	dhcp []string

	// hostsFile is the information from the hosts file.  nil indicates that
	// there is no information from the source.  Empty non-nil slice indicates
	// that the data from the source is present, but empty.
	hostsFile []string
}

// Info returns a client information from the highest-priority source.
func (r *Runtime) Info() (cs Source, host string) {
	info := []string{}

	switch {
	case r.hostsFile != nil:
		cs, info = SourceHostsFile, r.hostsFile
	case r.dhcp != nil:
		cs, info = SourceDHCP, r.dhcp
	case r.rdns != nil:
		cs, info = SourceRDNS, r.rdns
	case r.arp != nil:
		cs, info = SourceARP, r.arp
	case r.whois != nil:
		cs = SourceWHOIS
	}

	if len(info) == 0 {
		return cs, ""
	}

	// TODO(s.chzhen):  Return the full information.
	return cs, info[0]
}

// SetInfo sets a host as a client information from the cs.
func (r *Runtime) SetInfo(cs Source, hosts []string) {
	if len(hosts) == 1 && hosts[0] == "" {
		hosts = []string{}
	}

	switch cs {
	case SourceARP:
		r.arp = hosts
	case SourceRDNS:
		r.rdns = hosts
	case SourceDHCP:
		r.dhcp = hosts
	case SourceHostsFile:
		r.hostsFile = hosts
	}
}

// WHOIS returns a WHOIS client information.
func (r *Runtime) WHOIS() (info *whois.Info) {
	return r.whois
}

// SetWHOIS sets a WHOIS client information.  info must be non-nil.
func (r *Runtime) SetWHOIS(info *whois.Info) {
	r.whois = info
}

// Unset clears a cs information.
func (r *Runtime) Unset(cs Source) {
	switch cs {
	case SourceWHOIS:
		r.whois = nil
	case SourceARP:
		r.arp = nil
	case SourceRDNS:
		r.rdns = nil
	case SourceDHCP:
		r.dhcp = nil
	case SourceHostsFile:
		r.hostsFile = nil
	}
}

// IsEmpty returns true if there is no information from any source.
func (r *Runtime) IsEmpty() (ok bool) {
	return r.whois == nil &&
		r.arp == nil &&
		r.rdns == nil &&
		r.dhcp == nil &&
		r.hostsFile == nil
}
