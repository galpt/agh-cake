package configmgr

import (
	"fmt"
	"net/netip"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/timeutil"
)

// Configuration Structures

// config is the top-level on-disk configuration structure.
type config struct {
	DNS  *dnsConfig  `yaml:"dns"`
	HTTP *httpConfig `yaml:"http"`
	Log  *logConfig  `yaml:"log"`
	// TODO(a.garipov): Use.
	SchemaVersion int `yaml:"schema_version"`
}

const errNoConf errors.Error = "configuration not found"

// validate returns an error if the configuration structure is invalid.
func (c *config) validate() (err error) {
	if c == nil {
		return errNoConf
	}

	// TODO(a.garipov): Add more validations.

	// Keep this in the same order as the fields in the config.
	validators := []struct {
		validate func() (err error)
		name     string
	}{{
		validate: c.DNS.validate,
		name:     "dns",
	}, {
		validate: c.HTTP.validate,
		name:     "http",
	}, {
		validate: c.Log.validate,
		name:     "log",
	}}

	for _, v := range validators {
		err = v.validate()
		if err != nil {
			return fmt.Errorf("%s: %w", v.name, err)
		}
	}

	return nil
}

// dnsConfig is the on-disk DNS configuration.
type dnsConfig struct {
	Addresses           []netip.AddrPort  `yaml:"addresses"`
	BootstrapDNS        []string          `yaml:"bootstrap_dns"`
	UpstreamDNS         []string          `yaml:"upstream_dns"`
	DNS64Prefixes       []netip.Prefix    `yaml:"dns64_prefixes"`
	UpstreamTimeout     timeutil.Duration `yaml:"upstream_timeout"`
	BootstrapPreferIPv6 bool              `yaml:"bootstrap_prefer_ipv6"`
	UseDNS64            bool              `yaml:"use_dns64"`
}

// validate returns an error if the DNS configuration structure is invalid.
//
// TODO(a.garipov): Add more validations.
func (c *dnsConfig) validate() (err error) {
	// TODO(a.garipov): Add more validations.
	switch {
	case c == nil:
		return errNoConf
	case c.UpstreamTimeout.Duration <= 0:
		return newMustBePositiveError("upstream_timeout", c.UpstreamTimeout)
	default:
		return nil
	}
}

// httpConfig is the on-disk web API configuration.
type httpConfig struct {
	Pprof *httpPprofConfig `yaml:"pprof"`

	// TODO(a.garipov): Document the configuration change.
	Addresses       []netip.AddrPort  `yaml:"addresses"`
	SecureAddresses []netip.AddrPort  `yaml:"secure_addresses"`
	Timeout         timeutil.Duration `yaml:"timeout"`
	ForceHTTPS      bool              `yaml:"force_https"`
}

// validate returns an error if the HTTP configuration structure is invalid.
//
// TODO(a.garipov): Add more validations.
func (c *httpConfig) validate() (err error) {
	switch {
	case c == nil:
		return errNoConf
	case c.Timeout.Duration <= 0:
		return newMustBePositiveError("timeout", c.Timeout)
	default:
		return c.Pprof.validate()
	}
}

// httpPprofConfig is the on-disk pprof configuration.
type httpPprofConfig struct {
	Port    uint16 `yaml:"port"`
	Enabled bool   `yaml:"enabled"`
}

// validate returns an error if the pprof configuration structure is invalid.
func (c *httpPprofConfig) validate() (err error) {
	if c == nil {
		return errNoConf
	}

	return nil
}

// logConfig is the on-disk web API configuration.
type logConfig struct {
	// TODO(a.garipov): Use.
	Verbose bool `yaml:"verbose"`
}

// validate returns an error if the HTTP configuration structure is invalid.
//
// TODO(a.garipov): Add more validations.
func (c *logConfig) validate() (err error) {
	if c == nil {
		return errNoConf
	}

	return nil
}
