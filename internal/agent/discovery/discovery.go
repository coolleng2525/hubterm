// Package discovery provides auto-discovery of the HubTerm center service.
//
// Discovery strategies (in priority order):
//  1. DNS SRV record: _hubterm._tcp.<domain> → returns target:port
//  2. DNS A record: hubterm.<domain> → returns IP:8080
//  3. mDNS: hubterm.local LAN broadcast
//  4. Environment variable: HUBTERM_CENTER_URL
//  5. Direct config: pre-configured center URL
package discovery

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

var (
	lookupSRV  = net.LookupSRV
	lookupHost = net.LookupHost
)

// DiscoveryResult holds the result of a center service discovery.
type DiscoveryResult struct {
	// CenterURL is the discovered WebSocket URL of the center service.
	CenterURL string `json:"center_url"`
	// Domain is the domain used for discovery (may be empty).
	Domain string `json:"domain"`
	// Method indicates which discovery strategy succeeded.
	Method string `json:"method"` // dns_srv / dns_a / mdns / env / config
	// NodeToken is an optional pre-configured node token.
	NodeToken string `json:"node_token,omitempty"`
}

// Discover performs auto-discovery of the center service.
//
// Discovery strategies (in priority order):
//  1. DNS SRV record: _hubterm._tcp.<domain> → returns target:Port
//  2. DNS A record: hubterm.<domain> → returns IP:8080
//  3. mDNS: hubterm.local LAN broadcast
//  4. Environment variable: HUBTERM_CENTER_URL
//  5. Direct config: if domain is empty, falls back to env var
//
// Returns an error if all strategies fail.
func Discover(domain string) (*DiscoveryResult, error) {
	// If no domain is given, skip DNS strategies and check env var directly.
	if domain == "" {
		if envURL := os.Getenv("HUBTERM_CENTER_URL"); envURL != "" {
			return &DiscoveryResult{
				CenterURL: envURL,
				Method:    "env",
			}, nil
		}
		return nil, fmt.Errorf("discovery: no domain specified and HUBTERM_CENTER_URL not set")
	}

	// Strategy 1: DNS SRV record _hubterm._tcp.<domain>
	if result, err := discoverSRV(domain); err == nil {
		return result, nil
	}

	// Strategy 2: DNS A record hubterm.<domain>
	if result, err := discoverA(domain); err == nil {
		return result, nil
	}

	// Strategy 3: mDNS hubterm.local
	if result, err := discoverMDNS(); err == nil {
		return result, nil
	}

	// Strategy 4: Environment variable HUBTERM_CENTER_URL
	if envURL := os.Getenv("HUBTERM_CENTER_URL"); envURL != "" {
		return &DiscoveryResult{
			CenterURL: envURL,
			Domain:    domain,
			Method:    "env",
		}, nil
	}

	return nil, fmt.Errorf("discovery failed for domain %q: no SRV, A, or mDNS records found", domain)
}

// DiscoverWithTimeout performs auto-discovery with a configurable timeout.
//
// If the discovery does not complete within the given duration, an error is
// returned indicating a timeout.
func DiscoverWithTimeout(domain string, timeout time.Duration) (*DiscoveryResult, error) {
	type result struct {
		res *DiscoveryResult
		err error
	}
	ch := make(chan result, 1)
	go func() {
		res, err := Discover(domain)
		ch <- result{res, err}
	}()
	select {
	case r := <-ch:
		return r.res, r.err
	case <-time.After(timeout):
		return nil, fmt.Errorf("discovery timed out after %v", timeout)
	}
}

// DiscoverWithConfig returns a discovery result from a pre-configured center URL.
//
// This is the highest-priority strategy — if a URL is explicitly provided via
// command-line flag or config file, it is used directly without any network
// discovery.
func DiscoverWithConfig(centerURL string) (*DiscoveryResult, error) {
	if centerURL == "" {
		return nil, fmt.Errorf("discovery: center URL is empty")
	}
	return &DiscoveryResult{
		CenterURL: centerURL,
		Method:    "config",
	}, nil
}

// discoverSRV looks up the DNS SRV record _hubterm._tcp.<domain>.
// Returns an HTTP URL (e.g. http://center.mycompany.com:8080) that the
// connector will convert to a WebSocket URL internally.
func discoverSRV(domain string) (*DiscoveryResult, error) {
	_, srvs, err := lookupSRV("hubterm", "tcp", domain)
	if err != nil {
		return nil, fmt.Errorf("SRV lookup _hubterm._tcp.%s: %w", domain, err)
	}
	if len(srvs) == 0 {
		return nil, fmt.Errorf("SRV lookup _hubterm._tcp.%s: no records found", domain)
	}
	srv := srvs[0]
	target := strings.TrimSuffix(srv.Target, ".")
	centerURL := fmt.Sprintf("http://%s:%d", target, srv.Port)
	return &DiscoveryResult{
		CenterURL: centerURL,
		Domain:    domain,
		Method:    "dns_srv",
	}, nil
}

// discoverA looks up the DNS A record hubterm.<domain>.
// Defaults to port 8080.
func discoverA(domain string) (*DiscoveryResult, error) {
	host := fmt.Sprintf("hubterm.%s", domain)
	addrs, err := lookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("A lookup %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("A lookup %s: no addresses found", host)
	}
	centerURL := fmt.Sprintf("http://%s:8080", addrs[0])
	return &DiscoveryResult{
		CenterURL: centerURL,
		Domain:    domain,
		Method:    "dns_a",
	}, nil
}

// discoverMDNS looks up hubterm.local via mDNS.
func discoverMDNS() (*DiscoveryResult, error) {
	host := "hubterm.local"
	addrs, err := lookupHost(host)
	if err != nil {
		return nil, fmt.Errorf("mDNS lookup %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("mDNS lookup %s: no addresses found", host)
	}
	centerURL := fmt.Sprintf("http://%s:8080", addrs[0])
	return &DiscoveryResult{
		CenterURL: centerURL,
		Domain:    "local",
		Method:    "mdns",
	}, nil
}
