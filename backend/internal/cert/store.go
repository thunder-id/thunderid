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

package cert

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
)

// certificateStoreInterface defines the methods for certificate storage operations.
type certificateStoreInterface interface {
	GetCertificateByID(ctx context.Context, id string) (*Certificate, error)
	GetCertificateByReference(ctx context.Context, refType CertificateReferenceType, refID string) (*Certificate, error)
	CreateCertificate(ctx context.Context, cert *Certificate) error
	UpdateCertificateByID(ctx context.Context, existingCert, updatedCert *Certificate) error
	UpdateCertificateByReference(ctx context.Context, existingCert, updatedCert *Certificate) error
	DeleteCertificateByID(ctx context.Context, id string) error
	DeleteCertificateByReference(ctx context.Context, refType CertificateReferenceType, refID string) error
}

// certificateStore implements the certificateStoreInterface for managing certificates.
type certificateStore struct {
	dbProvider   dbprovider.DBProviderInterface
	deploymentID string
}

// NewCertificateStore creates a new instance of CertificateStore.
func newCertificateStore() certificateStoreInterface {
	return &certificateStore{
		dbProvider:   dbprovider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// GetCertificateByID retrieves a certificate by its ID.
func (s *certificateStore) GetCertificateByID(ctx context.Context, id string) (*Certificate, error) {
	return s.getCertificate(ctx, queryGetCertificateByID, id, s.deploymentID)
}

// GetCertificateByReference retrieves a certificate by its reference type and ID.
func (s *certificateStore) GetCertificateByReference(ctx context.Context, refType CertificateReferenceType,
	refID string) (*Certificate, error) {
	return s.getCertificate(ctx, queryGetCertificateByReference, refType, refID, s.deploymentID)
}

// getCertificate retrieves a certificate based on a query and its arguments.
func (s *certificateStore) getCertificate(ctx context.Context, query dbmodel.DBQuery,
	args ...interface{}) (*Certificate, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return nil, ErrCertificateNotFound
	}
	if len(results) > 1 {
		return nil, errors.New("multiple certificates found")
	}

	cert, err := s.buildCertificateFromResultRow(results[0])
	if err != nil {
		return nil, fmt.Errorf("failed to build certificate from result row: %w", err)
	}
	return cert, nil
}

// buildCertificateFromResultRow builds a Certificate object from a database result row.
func (s *certificateStore) buildCertificateFromResultRow(row map[string]interface{}) (*Certificate, error) {
	certID, ok := row["id"].(string)
	if !ok {
		return nil, errors.New("failed to parse id as string")
	}

	refTypeStr, ok := row["ref_type"].(string)
	if !ok {
		return nil, errors.New("failed to parse ref_type as string")
	}
	refType := CertificateReferenceType(refTypeStr)

	refID, ok := row["ref_id"].(string)
	if !ok {
		return nil, errors.New("failed to parse ref_id as string")
	}

	typeStr, ok := row["type"].(string)
	if !ok {
		return nil, errors.New("failed to parse type as string")
	}
	certType := CertificateType(typeStr)

	value, ok := row["value"].(string)
	if !ok {
		return nil, errors.New("failed to parse value as string")
	}

	return &Certificate{
		ID:      certID,
		RefType: refType,
		RefID:   refID,
		Type:    certType,
		Value:   value,
	}, nil
}

// CreateCertificate creates a new certificate in the database.
func (s *certificateStore) CreateCertificate(ctx context.Context, cert *Certificate) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rows, err := dbClient.ExecuteContext(ctx, queryInsertCertificate, cert.ID, cert.RefType, cert.RefID, cert.Type,
		cert.Value, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert certificate: %w", err)
	}
	if rows == 0 {
		return errors.New("no rows affected, certificate creation failed")
	}

	return nil
}

// UpdateCertificateByID updates a certificate by its ID.
func (s *certificateStore) UpdateCertificateByID(ctx context.Context, existingCert, updatedCert *Certificate) error {
	return s.updateCertificate(ctx, queryUpdateCertificateByID, existingCert.ID, updatedCert.Type, updatedCert.Value,
		s.deploymentID)
}

// UpdateCertificateByReference updates a certificate by its reference type and ID.
func (s *certificateStore) UpdateCertificateByReference(ctx context.Context,
	existingCert, updatedCert *Certificate) error {
	return s.updateCertificate(ctx, queryUpdateCertificateByReference, existingCert.RefType, existingCert.RefID,
		updatedCert.Type, updatedCert.Value, s.deploymentID)
}

// updateCertificate updates a certificate based on a query and its arguments.
func (s *certificateStore) updateCertificate(ctx context.Context, query dbmodel.DBQuery, args ...interface{}) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rows, err := dbClient.ExecuteContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update certificate: %w", err)
	}
	if rows == 0 {
		return errors.New("no rows affected, certificate update failed")
	}

	return nil
}

// DeleteCertificateByID deletes a certificate by its ID.
func (s *certificateStore) DeleteCertificateByID(ctx context.Context, id string) error {
	return s.deleteCertificate(ctx, queryDeleteCertificateByID, id, s.deploymentID)
}

// DeleteCertificateByReference deletes a certificate by its reference type and ID.
func (s *certificateStore) DeleteCertificateByReference(ctx context.Context, refType CertificateReferenceType,
	refID string) error {
	return s.deleteCertificate(ctx, queryDeleteCertificateByReference, refType, refID, s.deploymentID)
}

// deleteCertificate deletes a certificate based on a query and its arguments.
func (s *certificateStore) deleteCertificate(ctx context.Context, query dbmodel.DBQuery, args ...interface{}) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to execute delete query: %w", err)
	}

	return nil
}
