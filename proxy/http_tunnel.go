package proxy

import (
	"net"
	"net/url"

	"github.com/jim3ma/http-tunnel"
)

func httpTunnel(network, schema, addr string, auth *Auth, forward Dialer, resolver Resolver) (Dialer, error) {
	var host string
	var port string

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	d := ht.NewTunnel(schema, host, port, "/ping", "/pong", url.UserPassword(auth.User, auth.Password))

	return d, nil
}