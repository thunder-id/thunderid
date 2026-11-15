// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package verifier provides the registry of verifiable credential (VC) verifiers keyed by credential format.
package verifier

import (
	"errors"
	"fmt"
	"sync"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

var (
	// ErrInvalidRegistration is returned when a verifier is registered with an empty format or a nil verifier.
	ErrInvalidRegistration = errors.New("invalid vc verifier registration")
	// ErrVerifierAlreadyRegistered is returned when a verifier is already registered for the format.
	ErrVerifierAlreadyRegistered = errors.New("vc verifier already registered for format")
	// ErrVerifierNotFound is returned when no verifier is registered for the format.
	ErrVerifierNotFound = errors.New("no vc verifier registered for format")
)

// VCVerifierFactoryInterface resolves the VC verifier responsible for a credential format.
// Exactly one verifier is registered per format.
type VCVerifierFactoryInterface interface {
	// Register registers verifier as the verifier for format.
	Register(format string, verifier providers.VCVerifierProvider) error
	// GetVerifier returns the verifier registered for format.
	GetVerifier(format string) (providers.VCVerifierProvider, error)
}

var _ VCVerifierFactoryInterface = (*vcVerifierFactory)(nil)

type vcVerifierFactory struct {
	mu        sync.RWMutex
	verifiers map[string]providers.VCVerifierProvider
}

// NewVCVerifierFactory creates an empty VC verifier factory.
func NewVCVerifierFactory() VCVerifierFactoryInterface {
	return &vcVerifierFactory{verifiers: make(map[string]providers.VCVerifierProvider)}
}

// Register registers verifier for format, rejecting a second registration for the same format.
func (f *vcVerifierFactory) Register(format string, verifier providers.VCVerifierProvider) error {
	if format == "" || verifier == nil {
		return fmt.Errorf("%w: format and verifier are required", ErrInvalidRegistration)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.verifiers[format]; exists {
		return fmt.Errorf("%w: %q", ErrVerifierAlreadyRegistered, format)
	}
	f.verifiers[format] = verifier
	return nil
}

// GetVerifier returns the verifier registered for format.
func (f *vcVerifierFactory) GetVerifier(format string) (providers.VCVerifierProvider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	verifier, ok := f.verifiers[format]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrVerifierNotFound, format)
	}
	return verifier, nil
}
