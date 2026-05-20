/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package cert provides the implementation for managing certificates in the system.
package cert

import (
	"context"
	"errors"

	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "CertificateService"

// CertificateServiceInterface defines the methods for certificate service operations.
type CertificateServiceInterface interface {
	GetCertificateByID(ctx context.Context, id string) (*Certificate, *serviceerror.ServiceError)
	GetCertificateByReference(ctx context.Context, refType CertificateReferenceType, refID string) (
		*Certificate, *serviceerror.ServiceError)
	CreateCertificate(ctx context.Context, cert *Certificate) (*Certificate, *serviceerror.ServiceError)
	UpdateCertificateByID(ctx context.Context, id string, cert *Certificate) (
		*Certificate, *serviceerror.ServiceError)
	UpdateCertificateByReference(ctx context.Context, refType CertificateReferenceType, refID string,
		cert *Certificate) (*Certificate, *serviceerror.ServiceError)
	DeleteCertificateByID(ctx context.Context, id string) *serviceerror.ServiceError
	DeleteCertificateByReference(ctx context.Context, refType CertificateReferenceType,
		refID string) *serviceerror.ServiceError
}

// certificateService implements the CertificateServiceInterface for managing certificates.
type certificateService struct {
	store         certificateStoreInterface
	transactioner transaction.Transactioner
}

// newCertificateService creates a new instance of CertificateService.
func newCertificateService(store certificateStoreInterface,
	transactioner transaction.Transactioner) CertificateServiceInterface {
	return &certificateService{
		store:         store,
		transactioner: transactioner,
	}
}

// GetCertificateByID retrieves a certificate by its ID.
func (s *certificateService) GetCertificateByID(ctx context.Context,
	id string) (*Certificate, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if id == "" {
		return nil, &ErrorInvalidCertificateID
	}

	certObj, err := s.store.GetCertificateByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to get certificate by ID", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if certObj == nil {
		logger.Debug("Certificate not found for ID", log.String("id", id))
		return nil, &ErrorCertificateNotFound
	}

	return certObj, nil
}

// GetCertificateByReference retrieves a certificate by its reference type and ID.
func (s *certificateService) GetCertificateByReference(ctx context.Context, refType CertificateReferenceType,
	refID string) (*Certificate, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if !isValidReferenceType(refType) {
		return nil, &ErrorInvalidReferenceType
	}
	if refID == "" {
		return nil, &ErrorInvalidReferenceID
	}

	certObj, err := s.store.GetCertificateByReference(ctx, refType, refID)
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to get certificate by reference", log.String("refType", string(refType)),
			log.String("refID", refID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if certObj == nil {
		logger.Debug("Certificate not found for reference", log.String("refType", string(refType)),
			log.String("refID", refID))
		return nil, &ErrorCertificateNotFound
	}

	return certObj, nil
}

// CreateCertificate creates a new certificate.
func (s *certificateService) CreateCertificate(ctx context.Context, cert *Certificate) (*Certificate,
	*serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validateCertificateForCreation(cert); err != nil {
		return nil, err
	}

	// Check if a certificate with the same reference already exists
	existingCert, err := s.store.GetCertificateByReference(ctx, cert.RefType, cert.RefID)
	if err != nil && !errors.Is(err, ErrCertificateNotFound) {
		logger.Error("Failed to check existing certificate", log.String("refType", string(cert.RefType)),
			log.String("refID", cert.RefID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if existingCert != nil {
		return nil, &ErrorCertificateAlreadyExists
	}

	cert.ID, err = sysutils.GenerateUUIDv7()
	if err != nil {
		logger.Error("Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	err = s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.CreateCertificate(txCtx, cert)
	})
	if err != nil {
		logger.Error("Failed to create certificate", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return cert, nil
}

// UpdateCertificateByID updates an existing certificate by its ID.
func (s *certificateService) UpdateCertificateByID(ctx context.Context, id string, cert *Certificate) (
	*Certificate, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if id == "" {
		return nil, &ErrorInvalidCertificateID
	}
	if err := validateCertificate(cert); err != nil {
		return nil, err
	}

	// Get the existing certificate to validate reference
	existingCert, err := s.store.GetCertificateByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to get existing certificate", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if existingCert == nil {
		logger.Debug("Certificate not found for update", log.String("id", id))
		return nil, &ErrorCertificateNotFound
	}

	// Validate the reference is not changed
	if existingCert.RefType != cert.RefType || existingCert.RefID != cert.RefID {
		return nil, &ErrorReferenceUpdateIsNotAllowed
	}

	err = s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.UpdateCertificateByID(txCtx, existingCert, cert)
	})
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to update certificate by ID", log.String("id", id), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return cert, nil
}

// UpdateCertificateByReference updates an existing certificate by its reference type and ID.
func (s *certificateService) UpdateCertificateByReference(ctx context.Context, refType CertificateReferenceType,
	refID string, cert *Certificate) (*Certificate, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if !isValidReferenceType(refType) {
		return nil, &ErrorInvalidReferenceType
	}
	if refID == "" {
		return nil, &ErrorInvalidReferenceID
	}
	if err := validateCertificate(cert); err != nil {
		return nil, err
	}

	// Get the existing certificate to validate reference consistency
	existingCert, err := s.store.GetCertificateByReference(ctx, refType, refID)
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to get existing certificate", log.String("refType", string(refType)),
			log.String("refID", refID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if existingCert == nil {
		logger.Debug("Certificate not found for update", log.String("refType", string(refType)),
			log.String("refID", refID))
		return nil, &ErrorCertificateNotFound
	}

	// Validate the reference is not changed
	if existingCert.RefType != cert.RefType || existingCert.RefID != cert.RefID {
		return nil, &ErrorReferenceUpdateIsNotAllowed
	}

	cert.ID = existingCert.ID
	err = s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.UpdateCertificateByReference(txCtx, existingCert, cert)
	})
	if err != nil {
		if errors.Is(err, ErrCertificateNotFound) {
			return nil, &ErrorCertificateNotFound
		}
		logger.Error("Failed to update certificate by reference", log.String("refType", string(refType)),
			log.String("refID", refID), log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return cert, nil
}

// DeleteCertificateByID deletes a certificate by its ID.
func (s *certificateService) DeleteCertificateByID(ctx context.Context, id string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if id == "" {
		return &ErrorInvalidCertificateID
	}

	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.DeleteCertificateByID(txCtx, id)
	})
	if err != nil {
		logger.Error("Failed to delete certificate by ID", log.String("id", id), log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}

// DeleteCertificateByReference deletes a certificate by its reference type and ID.
func (s *certificateService) DeleteCertificateByReference(ctx context.Context, refType CertificateReferenceType,
	refID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if !isValidReferenceType(refType) {
		return &ErrorInvalidReferenceType
	}
	if refID == "" {
		return &ErrorInvalidReferenceID
	}

	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.DeleteCertificateByReference(txCtx, refType, refID)
	})
	if err != nil {
		logger.Error("Failed to delete certificate by reference", log.String("refType", string(refType)),
			log.String("refID", refID), log.Error(err))
		return &serviceerror.InternalServerError
	}

	return nil
}

// isValidReferenceType checks if the provided reference type is valid.
func isValidReferenceType(refType CertificateReferenceType) bool {
	switch refType {
	case CertificateReferenceTypeApplication, CertificateReferenceTypeIDP, CertificateReferenceTypeOAuthApp:
		return true
	default:
		return false
	}
}

// isValidCertificateType checks if the provided certificate type is valid.
func isValidCertificateType(certType CertificateType) bool {
	switch certType {
	case CertificateTypeJWKS, CertificateTypeJWKSURI:
		return true
	default:
		return false
	}
}

// validateCertificate checks if the provided certificate is valid.
func validateCertificate(cert *Certificate) *serviceerror.ServiceError {
	if cert == nil {
		return &ErrorInvalidCertificateValue
	}
	if cert.ID == "" {
		return &ErrorInvalidCertificateID
	}
	if cert.RefID == "" {
		return &ErrorInvalidReferenceID
	}
	if !isValidReferenceType(cert.RefType) {
		return &ErrorInvalidReferenceType
	}
	if !isValidCertificateType(cert.Type) {
		return &ErrorInvalidCertificateType
	}
	if len(cert.Value) < 10 || len(cert.Value) > 4096 {
		return &ErrorInvalidCertificateValue
	}
	return nil
}

// validateCertificateForCreation checks if the provided certificate is valid for creation.
func validateCertificateForCreation(cert *Certificate) *serviceerror.ServiceError {
	if cert == nil {
		return &ErrorInvalidCertificateValue
	}
	if cert.RefID == "" {
		return &ErrorInvalidReferenceID
	}
	if !isValidReferenceType(cert.RefType) {
		return &ErrorInvalidReferenceType
	}
	if !isValidCertificateType(cert.Type) {
		return &ErrorInvalidCertificateType
	}
	if len(cert.Value) < 10 || len(cert.Value) > 4096 {
		return &ErrorInvalidCertificateValue
	}
	return nil
}
