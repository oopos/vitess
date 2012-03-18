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

package umgmt

import (
	"code.google.com/p/vitess/go/relog"
	"testing"
	"time"
)

func serve() {
	AddShutdownCallback(ShutdownCallback(func() error { relog.Error("testserver GracefulShutdown callback"); return nil }))
	err := ListenAndServe("/tmp/test-sock")
	if err != nil {
		relog.Fatal("listen err:%v", err)
	}
	relog.Info("test server finished")
}

func TestUmgmt(t *testing.T) {
	go serve()
	time.Sleep(1e9)

	client, err := Dial("/tmp/test-sock")
	if err != nil {
		t.Fatalf("can't connect %v", err)
	}
	request := new(Request)
	reply := new(Reply)

	callErr := client.Call("UmgmtService.Ping", request, reply)
	if callErr != nil {
		t.Fatalf("callErr: %v", callErr)
	}
	relog.Info("reply: %v", reply.Message)

	callErr = client.Call("UmgmtService.CloseListeners", reply, reply)
	if callErr != nil {
		t.Fatalf("callErr: %v", callErr)
	}
	relog.Info("reply: %v", reply.Message)
	time.Sleep(5e9)
	callErr = client.Call("UmgmtService.GracefulShutdown", reply, reply)
	if callErr != nil {
		t.Fatalf("callErr: %v", callErr)
	}
	relog.Info("reply: %v", reply.Message)
}
