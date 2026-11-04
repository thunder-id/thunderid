// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// BuildRedisTLSConfig builds the client TLS configuration for a Redis connection.
// It returns nil when TLS is not enabled.
func BuildRedisTLSConfig(cfg engineconfig.RedisTLSConfig) (*tls.Config, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return nil, errors.New("redis tls cert_file and key_file must be set together")
	}

	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         cfg.ServerName,
		InsecureSkipVerify: cfg.InsecureSkipVerify, //nolint:gosec // opt-in, for non-production setups only
	}

	if cfg.CAFile != "" {
		caPEM, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read redis tls ca_file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("redis tls ca_file contains no valid PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}

	if cfg.CertFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load redis tls client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}
