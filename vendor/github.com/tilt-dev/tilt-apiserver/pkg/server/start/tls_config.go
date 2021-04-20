// Adapted from
// https://github.com/kubernetes/apiserver/blob/4b2cf85d1be8a30278fc61c27f9f79d0d3d827eb/pkg/server/secure_serving.go
// but with customized warnings

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

package start

import (
	"crypto/tls"
	"fmt"

	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/component-base/cli/flag"
)

type Warn interface {
	Warnf(tpl string, args ...interface{})
}

type WarnFunc func(tpl string, args ...interface{})

func (f WarnFunc) Warnf(tpl string, args ...interface{}) {
	f(tpl, args...)
}

// tlsConfig produces the tls.Config to serve with.
//
// Loads all the certs once at startup, then never reloads them again.
// This is different than a typical Kubernetes config, which periodically
// checks for cert updates.
func TLSConfig(s *server.SecureServingInfo) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		// Can't use SSLv3 because of POODLE and BEAST
		// Can't use TLSv1.0 because of POODLE and BEAST using CBC cipher
		// Can't use TLSv1.1 because of RC4 cipher usage
		MinVersion: tls.VersionTLS12,
		// enable HTTP2 for go's 1.7 HTTP Server
		NextProtos: []string{"h2", "http/1.1"},
	}

	// these are static aspects of the tls.Config
	if s.DisableHTTP2 {
		tlsConfig.NextProtos = []string{"http/1.1"}
	}
	if s.MinTLSVersion > 0 {
		tlsConfig.MinVersion = s.MinTLSVersion
	}
	if len(s.CipherSuites) > 0 {
		tlsConfig.CipherSuites = s.CipherSuites
		insecureCiphers := flag.InsecureTLSCiphers()
		for i := 0; i < len(s.CipherSuites); i++ {
			for cipherName, cipherID := range insecureCiphers {
				if s.CipherSuites[i] == cipherID {
					return nil, fmt.Errorf("Use of insecure cipher '%s' detected.", cipherName)
				}
			}
		}
	}

	if s.ClientCA != nil {
		// Populate PeerCertificates in requests, but don't reject connections without certificates
		// This allows certificates to be validated by authenticators, while still allowing other auth types
		tlsConfig.ClientAuth = tls.RequestClientCert
	}

	if s.ClientCA != nil || s.Cert != nil || len(s.SNICerts) > 0 {
		dynamicCertificateController := dynamiccertificates.NewDynamicServingCertificateController(
			tlsConfig,
			s.ClientCA,
			s.Cert,
			s.SNICerts,
			nil, // TODO see how to plumb an event recorder down in here. For now this results in simply klog messages.
		)
		// register if possible
		if notifier, ok := s.ClientCA.(dynamiccertificates.Notifier); ok {
			notifier.AddListener(dynamicCertificateController)
		}
		if notifier, ok := s.Cert.(dynamiccertificates.Notifier); ok {
			notifier.AddListener(dynamicCertificateController)
		}
		// start controllers if possible
		if controller, ok := s.ClientCA.(dynamiccertificates.ControllerRunner); ok {
			// runonce to try to prime data.  If this fails, it's ok because we fail closed.
			// Files are required to be populated already, so this is for convenience.
			if err := controller.RunOnce(); err != nil {
				return nil, fmt.Errorf("Initial population of client CA failed: %v", err)
			}
		}
		if controller, ok := s.Cert.(dynamiccertificates.ControllerRunner); ok {
			// runonce to try to prime data.  If this fails, it's ok because we fail closed.
			// Files are required to be populated already, so this is for convenience.
			if err := controller.RunOnce(); err != nil {
				return nil, fmt.Errorf("Initial population of default serving certificate failed: %v", err)
			}
		}
		for _, sniCert := range s.SNICerts {
			if notifier, ok := sniCert.(dynamiccertificates.Notifier); ok {
				notifier.AddListener(dynamicCertificateController)
			}

			if controller, ok := sniCert.(dynamiccertificates.ControllerRunner); ok {
				// runonce to try to prime data.  If this fails, it's ok because we fail closed.
				// Files are required to be populated already, so this is for convenience.
				if err := controller.RunOnce(); err != nil {
					return nil, fmt.Errorf("Initial population of SNI serving certificate failed: %v", err)
				}
			}
		}

		// runonce to try to prime data.  If this fails, it's ok because we fail closed.
		// Files are required to be populated already, so this is for convenience.
		if err := dynamicCertificateController.RunOnce(); err != nil {
			return nil, fmt.Errorf("Initial population of dynamic certificates failed: %v", err)
		}

		tlsConfig.GetConfigForClient = dynamicCertificateController.GetConfigForClient
	}

	return tlsConfig, nil
}
