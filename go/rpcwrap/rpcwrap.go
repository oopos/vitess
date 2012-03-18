/*
Copyright 2012, Google Inc.
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

    * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
    * Neither the name of Google Inc. nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,           
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY           
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

package rpcwrap

import (
	"bufio"
	"errors"
	"io"
	"net"
	"net/http"
	"net/rpc"

	"code.google.com/p/vitess/go/relog"
)

const (
	connected = "200 Connected to Go RPC"
)

type ClientCodecFactory func(conn io.ReadWriteCloser) rpc.ClientCodec

type BufferedConnection struct {
	*bufio.Reader
	io.WriteCloser
}

func NewBufferedConnection(conn io.ReadWriteCloser) *BufferedConnection {
	return &BufferedConnection{bufio.NewReader(conn), conn}
}

// DialHTTP connects to a go HTTP RPC server using the specified codec.
func DialHTTP(network, address, codecName string, cFactory ClientCodecFactory) (*rpc.Client, error) {
	var err error
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	io.WriteString(conn, "CONNECT "+GetRpcPath(codecName)+" HTTP/1.0\n\n")

	// Require successful HTTP response
	// before switching to RPC protocol.
	buffered := NewBufferedConnection(conn)
	resp, err := http.ReadResponse(buffered.Reader, &http.Request{Method: "CONNECT"})
	if err == nil && resp.Status == connected {
		return rpc.NewClientWithCodec(cFactory(buffered)), nil
	}
	if err == nil {
		err = errors.New("unexpected HTTP response: " + resp.Status)
	}
	conn.Close()
	return nil, &net.OpError{"dial-http", network + " " + address, nil, err}
}

type ServerCodecFactory func(conn io.ReadWriteCloser) rpc.ServerCodec

// ServeRPC handles rpc requests using the hijack scheme of rpc
func ServeRPC(codecName string, cFactory ServerCodecFactory) {
	http.Handle(GetRpcPath(codecName), &rpcHandler{cFactory})
}

// ServeHTTP handles rpc requests in HTTP compliant POST form
func ServeHTTP(codecName string, cFactory ServerCodecFactory) {
	http.Handle(GetHttpPath(codecName), &httpHandler{cFactory})
}

type rpcHandler struct {
	cFactory ServerCodecFactory
}

func (self *rpcHandler) ServeHTTP(c http.ResponseWriter, req *http.Request) {
	conn, _, err := c.(http.Hijacker).Hijack()
	if err != nil {
		relog.Error("rpc hijacking %s: %v", req.RemoteAddr, err)
		return
	}
	io.WriteString(conn, "HTTP/1.0 "+connected+"\n\n")
	rpc.ServeCodec(self.cFactory(NewBufferedConnection(conn)))
}

func GetRpcPath(codecName string) string {
	return "/_" + codecName + "_rpc_"
}

type httpHandler struct {
	cFactory ServerCodecFactory
}

func (self *httpHandler) ServeHTTP(c http.ResponseWriter, req *http.Request) {
	conn := &httpConnectionBroker{c, req.Body}
	codec := self.cFactory(conn)
	if err := rpc.ServeRequest(codec); err != nil {
		relog.Error("rpcwrap: %v", err)
	}
}

// Emulate a read/write connection for the server codec
type httpConnectionBroker struct {
	http.ResponseWriter
	io.Reader
}

func (self *httpConnectionBroker) Close() error {
	return nil
}

func GetHttpPath(codecName string) string {
	return "/_" + codecName + "_http_"
}
