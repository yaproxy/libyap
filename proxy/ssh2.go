// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package proxy

import (
	"errors"
	"net"
	"os"
	"time"

	"go.uber.org/atomic"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type SSH2Option func(*ssh2) error

func SSH2(network, addr string, auth *Auth, forward Dialer, resolver Resolver, opts ...SSH2Option) (Dialer, error) {
	s := &ssh2{
		network:  network,
		addr:     addr,
		forward:  forward,
		resolver: resolver,
		user:     auth.User,
		password: auth.Password,
	}

	s.sshDialer = atomic.NewPointer[sshDialer](nil)

	for _, opt := range opts {
		err := opt(s)
		if err != nil {
			return nil, err
		}
	}

	return s, nil
}

func SSH2WithPublicKeys(keys ...string) SSH2Option {
	return func(s *ssh2) error {
		var signers []ssh.Signer
		for _, p := range keys {
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			signer, err := ssh.ParsePrivateKey(data)
			if err != nil {
				return err
			}
			signers = append(signers, signer)
		}
		s.signers = signers
		return nil
	}
}

func SSH2WithKnownHosts(hostFiles ...string) SSH2Option {
	return func(s *ssh2) error {
		hostKeyCallback, err := knownhosts.New(hostFiles...)
		if err != nil {
			return err
		}
		s.hostKeyCallback = hostKeyCallback
		return nil
	}
}

func SSH2WithSkipKnownHosts() SSH2Option {
	return func(s *ssh2) error {
		s.hostKeyCallback = ssh.InsecureIgnoreHostKey()
		return nil
	}
}

type ssh2 struct {
	user, password  string
	signers         []ssh.Signer
	hostKeyCallback ssh.HostKeyCallback

	network, addr string
	forward       Dialer
	resolver      Resolver
	sshDialer     *atomic.Pointer[sshDialer]
}

type sshDialer struct {
	alive  *atomic.Bool
	client *ssh.Client
}

func (s *sshDialer) keepAlive(done <-chan struct{}) error {
	const keepAliveInterval = time.Minute
	t := time.NewTicker(keepAliveInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			_, _, err := s.client.SendRequest(
				"keepalive@golang.org", true, nil)
			if err != nil {
				s.alive.Store(false)
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (s *ssh2) newDialer() (*sshDialer, error) {
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.password),
		},
	}

	if len(s.signers) > 0 {
		config.Auth = append(config.Auth, ssh.PublicKeys(s.signers...))
		config.HostKeyCallback = s.hostKeyCallback
	}

	sshClient, err := ssh.Dial(s.network, s.addr, config)
	if err != nil {
		return nil, err
	}

	dialer := &sshDialer{
		alive:  atomic.NewBool(true),
		client: sshClient,
	}

	// TODO
	done := make(chan struct{})
	go dialer.keepAlive(done)
	return dialer, nil
}

// Dial connects to the address addr on the network net via the HTTP1 proxy.
func (s *ssh2) Dial(network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp6", "tcp4":
	default:
		return nil, errors.New("proxy: no support for HTTP proxy connections of type " + network)
	}

	// dialer is nil or is not alive
	if s.sshDialer.Load() == nil || !s.sshDialer.Load().alive.Load() {
		d, err := s.newDialer()
		if err != nil {
			return nil, err
		}
		s.sshDialer.Store(d)
	}

	dialer := s.sshDialer.Load().client
	return dialer.Dial(network, addr)
}
