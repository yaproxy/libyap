// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package proxy provides support for a variety of protocols to proxy network
// data.
package proxy

import (
	"errors"
	"net"
	"net/url"
	"os"
)

// A Dialer is a means to establish a connection.
type Dialer interface {
	// Dial connects to the given address via the proxy.
	Dial(network, addr string) (c net.Conn, err error)
}

// A Resolver is a means to transform hostname.
type Resolver interface {
	LookupHost(host string) (addrs []string, err error)
}

// Auth contains authentication parameters that specific Dialers may require.
type Auth struct {
	User, Password string
}

// FromEnvironment returns the dialer specified by the proxy related variables in
// the environment.
func FromEnvironment() Dialer {
	allProxy := os.Getenv("all_proxy")
	if len(allProxy) == 0 {
		return Direct
	}

	proxyURL, err := url.Parse(allProxy)
	if err != nil {
		return Direct
	}
	proxy, err := FromURL(proxyURL, Direct, DummyResolver)
	if err != nil {
		return Direct
	}

	noProxy := os.Getenv("no_proxy")
	if len(noProxy) == 0 {
		return proxy
	}

	perHost := NewPerHost(proxy, Direct)
	perHost.AddFromString(noProxy)
	return perHost
}

// proxySchemes is a map from URL schemes to a function that creates a Dialer
// from a URL with such a scheme.
var proxySchemes map[string]func(*url.URL, Dialer) (Dialer, error)

// RegisterDialerType takes a URL scheme and a function to generate Dialers from
// a URL with that scheme and a forwarding Dialer. Registered schemes are used
// by FromURL.
func RegisterDialerType(scheme string, f func(*url.URL, Dialer) (Dialer, error)) {
	if proxySchemes == nil {
		proxySchemes = make(map[string]func(*url.URL, Dialer) (Dialer, error))
	}
	proxySchemes[scheme] = f
}

// FromURL returns a Dialer given a URL specification and an underlying
// Dialer for it to make network requests.
func FromURL(u *url.URL, forward Dialer, resolver Resolver) (Dialer, error) {
	var auth *Auth
	if u.User != nil {
		auth = new(Auth)
		auth.User = u.User.Username()
		if p, ok := u.User.Password(); ok {
			auth.Password = p
		}
	}

	switch u.Scheme {
	case "socks5", "socks":
		return SOCKS5("tcp", u.Host, auth, forward, resolver)
	case "socks4":
		return SOCKS4("tcp", u.Host, false, forward, resolver)
	case "socks4a":
		return SOCKS4("tcp", u.Host, true, forward, resolver)
	case "http":
		return HTTP1("tcp", u.Host, auth, forward, resolver)
	case "https":
		return HTTPS("tcp", u.Host, auth, forward, resolver)
	case "https+h2":
		return HTTP2("tcp", u.Host, auth, forward, resolver)
	case "ssh", "ssh2":
		query := u.Query()
		var opts []SSH2Option
		if v, ok := query["keys"]; ok {
			opts = append(opts, SSH2WithPublicKeys(v...))
		}
		if v, ok := query["key"]; ok {
			opts = append(opts, SSH2WithPublicKeys(v...))
		}
		if _, ok := query["skip_known_hosts"]; ok {
			opts = append(opts, SSH2WithSkipKnownHosts())
		}
		if v, ok := query["known_hosts"]; ok {
			opts = append(opts, SSH2WithKnownHosts(v...))
		}
		return SSH2("tcp", u.Host, auth, forward, resolver, opts...)
	case "http+post":
		return httpTunnel("tcp", "http", u.Host, auth, forward, resolver)
	case "https+post":
		return httpTunnel("tcp", "https", u.Host, auth, forward, resolver)
	}

	// If the scheme doesn't match any of the built-in schemes, see if it
	// was registered by another package.
	if proxySchemes != nil {
		if f, ok := proxySchemes[u.Scheme]; ok {
			return f(u, forward)
		}
	}

	return nil, errors.New("proxy: unknown scheme: " + u.Scheme)
}
