// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package cryptolib

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"

	"golang.org/x/crypto/argon2"
)

const (
	maxUint8  = int(^uint8(0))
	maxUint32 = int(^uint32(0))
)

// CredAlgorithm represents the supported credential hashing algorithms.
type CredAlgorithm string

const (
	// SHA256 represents the SHA-256 hashing algorithm.
	SHA256 CredAlgorithm = "SHA256"
	// PBKDF2 represents the PBKDF2 key derivation function.
	PBKDF2 CredAlgorithm = "PBKDF2"
	// ARGON2ID represents the Argon2id key derivation function.
	ARGON2ID CredAlgorithm = "ARGON2ID"
)

// CredParameters holds the parameters for credential hashing algorithms.
type CredParameters struct {
	Iterations  int
	Parallelism int
	Memory      int
	KeySize     int
	Salt        string
}

// Credential represents the output of a credential hash operation.
type Credential struct {
	Hash       string
	Parameters CredParameters
	Algorithm  CredAlgorithm
}

// HashAlgorithm represents the supported generic hash algorithms.
type HashAlgorithm string

const (
	// GenericSHA256 represents the SHA-256 hash algorithm.
	GenericSHA256 HashAlgorithm = "SHA-256"
	// GenericSHA384 represents the SHA-384 hash algorithm.
	GenericSHA384 HashAlgorithm = "SHA-384"
	// GenericSHA512 represents the SHA-512 hash algorithm.
	GenericSHA512 HashAlgorithm = "SHA-512"
)

// HashConfig holds all parameters needed to initialize the hash service.
// All configuration is provided by the caller (typically the key management layer);
// no config system is read from within this package.
type HashConfig struct {
	Algorithm   CredAlgorithm
	Parallelism int
	Memory      int
	SaltSize    int
	Iterations  int
	KeySize     int
}

// HashServiceInterface defines the interface for credential hashing services.
type HashServiceInterface interface {
	Generate(credentialValue []byte) (Credential, error)
	Verify(credentialValueToVerify []byte, referenceCredential Credential) (bool, error)
}

// Initialize returns a HashServiceInterface configured according to cfg.
// All hash algorithm parameters must be provided by the caller; this package reads no config.
func Initialize(cfg HashConfig) (HashServiceInterface, error) {
	return newHashService(cfg)
}

// hashService generates credentials with the configured algorithm and verifies each credential
// with the algorithm and parameters it was stored with.
type hashService struct {
	algorithm  CredAlgorithm
	saltSize   int
	parameters CredParameters
}

// newHashService validates cfg and returns a hash service that generates credentials with it.
func newHashService(cfg HashConfig) (HashServiceInterface, error) {
	var parameters CredParameters
	switch cfg.Algorithm {
	case SHA256:
	case PBKDF2:
		parameters = CredParameters{Iterations: cfg.Iterations, KeySize: cfg.KeySize}
	case ARGON2ID:
		parameters = CredParameters{
			Memory:      cfg.Memory,
			Iterations:  cfg.Iterations,
			Parallelism: cfg.Parallelism,
			KeySize:     cfg.KeySize,
		}
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", cfg.Algorithm)
	}
	if err := validatePositiveInt(cfg.SaltSize, "salt size"); err != nil {
		return nil, err
	}
	if err := validateParameters(cfg.Algorithm, parameters); err != nil {
		return nil, err
	}
	return &hashService{algorithm: cfg.Algorithm, saltSize: cfg.SaltSize, parameters: parameters}, nil
}

// Generate hashes the credential with a fresh salt using the configured algorithm.
func (s *hashService) Generate(credentialValue []byte) (Credential, error) {
	salt, err := generateSalt(s.saltSize)
	if err != nil {
		return Credential{}, err
	}
	h, err := deriveKey(credentialValue, s.algorithm, s.parameters, salt)
	if err != nil {
		return Credential{}, err
	}
	parameters := s.parameters
	parameters.Salt = hex.EncodeToString(salt)
	return Credential{
		Algorithm:  s.algorithm,
		Hash:       hex.EncodeToString(h),
		Parameters: parameters,
	}, nil
}

// Verify checks the credential against the reference using the reference credential's own algorithm
// and parameters, independent of the configured algorithm.
func (s *hashService) Verify(credentialValueToVerify []byte, referenceCredential Credential) (bool, error) {
	if err := validateParameters(referenceCredential.Algorithm, referenceCredential.Parameters); err != nil {
		return false, err
	}
	salt, err := decodeSalt(referenceCredential.Parameters.Salt)
	if err != nil {
		return false, err
	}
	referenceHash, err := hex.DecodeString(referenceCredential.Hash)
	if err != nil {
		return false, err
	}
	h, err := deriveKey(credentialValueToVerify, referenceCredential.Algorithm, referenceCredential.Parameters, salt)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(h, referenceHash) == 1, nil
}

// validateParameters checks that the parameters the algorithm uses are within supported bounds.
func validateParameters(algorithm CredAlgorithm, parameters CredParameters) error {
	switch algorithm {
	case SHA256:
		return nil
	case PBKDF2:
		if err := validatePositiveInt(parameters.Iterations, "iterations"); err != nil {
			return err
		}
		return validatePositiveInt(parameters.KeySize, "key size")
	case ARGON2ID:
		if err := validatePositiveIntWithMax(parameters.Memory, maxUint32, "memory"); err != nil {
			return err
		}
		if err := validatePositiveIntWithMax(parameters.Iterations, maxUint32, "iterations"); err != nil {
			return err
		}
		if err := validatePositiveIntWithMax(parameters.Parallelism, maxUint8, "parallelism"); err != nil {
			return err
		}
		return validatePositiveIntWithMax(parameters.KeySize, maxUint32, "key size")
	default:
		return fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}

// deriveKey hashes the credential value with the salt using the given algorithm and validated parameters.
func deriveKey(credentialValue []byte, algorithm CredAlgorithm, parameters CredParameters,
	salt []byte) ([]byte, error) {
	switch algorithm {
	case SHA256:
		credentialWithSalt := append([]byte(nil), credentialValue...)
		credentialWithSalt = append(credentialWithSalt, salt...)
		h := sha256.Sum256(credentialWithSalt)
		return h[:], nil
	case PBKDF2:
		return pbkdf2.Key(sha256.New, string(credentialValue), salt, parameters.Iterations, parameters.KeySize)
	case ARGON2ID:
		//nolint:gosec // G115 - Conversion is safe
		return argon2.IDKey(
			credentialValue,
			salt,
			uint32(parameters.Iterations),
			uint32(parameters.Memory),
			uint8(parameters.Parallelism),
			uint32(parameters.KeySize),
		), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", algorithm)
	}
}

// GenerateThumbprint generates a SHA-256 thumbprint for the given data.
func GenerateThumbprint(data []byte) string {
	h := sha256.Sum256(data)
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// GenerateThumbprintFromString generates a SHA-256 thumbprint for the given string data.
func GenerateThumbprintFromString(data string) string {
	return GenerateThumbprint([]byte(data))
}

// Hash returns the hash of the given data using the specified algorithm.
func Hash(data []byte, alg HashAlgorithm) ([]byte, error) {
	switch alg {
	case GenericSHA256:
		h := sha256.Sum256(data)
		return h[:], nil
	case GenericSHA384:
		h := sha512.Sum384(data)
		return h[:], nil
	case GenericSHA512:
		h := sha512.Sum512(data)
		return h[:], nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", alg)
	}
}

// GetHash returns a hash.Hash for the given algorithm.
func GetHash(alg HashAlgorithm) (hash.Hash, error) {
	switch alg {
	case GenericSHA256:
		return sha256.New(), nil
	case GenericSHA384:
		return sha512.New384(), nil
	case GenericSHA512:
		return sha512.New(), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", alg)
	}
}

func generateSalt(saltSize int) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

func decodeSalt(salt string) ([]byte, error) {
	if salt == "" {
		return nil, fmt.Errorf("salt must be provided")
	}
	return hex.DecodeString(salt)
}

// validatePositiveInt checks that the value is greater than zero.
func validatePositiveInt(value int, name string) error {
	if value <= 0 {
		return fmt.Errorf("%s must be positive", name)
	}
	return nil
}

// validatePositiveIntWithMax checks that the value is greater than zero and at most maxValue.
func validatePositiveIntWithMax(value, maxValue int, name string) error {
	if err := validatePositiveInt(value, name); err != nil {
		return err
	}
	if value > maxValue {
		return fmt.Errorf("%s exceeds maximum supported value", name)
	}
	return nil
}
