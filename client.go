// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
// Modify by linuz.ly

package sshclient

import (
	"bytes"
	"fmt"
	"net"
	"time"

	"code.google.com/p/go.crypto/ssh"
)

const (
	termType = "xterm"
)

type clientPassword string

func (p clientPassword) Password(user string) (string, error) {
	return string(p), nil
}

type Results struct {
	err    error
	rc     int
	stdout string
	stderr string
}

func DialPassword(server, username, password string, timeout int) (*ssh.Client, error) {
	// To authenticate with the remote server you must pass at least one
	// implementation of ClientAuth via the Auth field in ClientConfig.
	// Currently only the "password" authentication method is supported.

	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			// ClientAuthPassword wraps a ClientPassword implementation
			// in a type that implements ClientAuth.
			ssh.Password(password),
		},
	}
	conn, err := net.DialTimeout("tcp", server, time.Duration(timeout)*time.Second)
	if err != nil {
		return nil, err
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, server, config)
	if err != nil {
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func Run(client *ssh.Client, cmd string) Results {
	session, err := client.NewSession()
	if err != nil {
		return Results{err: err}
	}
	defer session.Close()

	// Set up terminal modes
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // disable echoing
		ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}
	// Request pseudo terminal
	if err := session.RequestPty(termType, 80, 40, modes); err != nil {
		return Results{err: fmt.Errorf("request for pseudo terminal failed: %s", err.Error())}
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr
	rc := 0
	if err := session.Run(cmd); err != nil {
		if err2, ok := err.(*ssh.ExitError); ok {
			rc = err2.Waitmsg.ExitStatus()
		}
	}
	return Results{nil, rc, stdout.String(), stderr.String()}
}


func Exec(server, username, password, cmd string, timeout int) (rc int, stdout, stderr string, err error) {
    var client *ssh.Client
    client, err = DialPassword(server, username, password, timeout)
    defer client.Close()
    if err != nil {
        return
    }

	c := make(chan Results)
	go func() {
        c <- Run(client, cmd)
    }()

	for {
		select {
		case r := <-c:
			err, rc, stdout, stderr = r.err, r.rc, r.stdout, r.stderr
			return
		case <-time.After(time.Duration(timeout) * time.Second):
			err = fmt.Errorf("Command timed out after %d seconds", timeout)
			return
		}
	}
}
