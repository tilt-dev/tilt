package server

/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"

	"github.com/tilt-dev/tilt/pkg/logger"
)

// This code has been adapted from
// https://github.com/kubernetes/apiserver/blob/master/pkg/server/secure_serving.go

// RunServer spawns a go-routine continuously serving
func runServer(
	ctx context.Context,
	server *http.Server,
	ln net.Listener,
) {
	go func() {
		defer runtime.HandleCrash()

		listener := tcpKeepAliveListener{ln}
		err := server.Serve(listener)
		msg := fmt.Sprintf("Stopped listening on %s", ln.Addr().String())
		select {
		case <-ctx.Done():
		default:
			logger.Get(ctx).Errorf("%s due to error: %v", msg, err)
		}
	}()
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
//
// Copied from Go 1.7.2 net/http/server.go
const (
	defaultKeepAlivePeriod = 3 * time.Minute
)

type tcpKeepAliveListener struct {
	net.Listener
}

func (ln tcpKeepAliveListener) Accept() (net.Conn, error) {
	c, err := ln.Listener.Accept()
	if err != nil {
		return nil, err
	}
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetKeepAlive(true)
		_ = tc.SetKeepAlivePeriod(defaultKeepAlivePeriod)
	}
	return c, nil
}
