/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

type sha256HashProvider struct {
	SaltSize int
}

type pbkdf2HashProvider struct {
	SaltSize   int
	Iterations int
	KeySize    int
}

type argon2idHashProvider struct {
	SaltSize    int
	Memory      int
	Iterations  int
	Parallelism int
	KeySize     int
}

func newHashService(cfg HashConfig) (HashServiceInterface, error) {
	switch cfg.Algorithm {
	case SHA256:
		if err := validatePositiveInt(cfg.SaltSize, "salt size"); err != nil {
			return nil, err
		}
		return newSHA256Provider(cfg.SaltSize), nil
	case PBKDF2:
		if err := validatePositiveInt(cfg.SaltSize, "salt size"); err != nil {
			return nil, err
		}
		if err := validatePositiveInt(cfg.Iterations, "iterations"); err != nil {
			return nil, err
		}
		if err := validatePositiveInt(cfg.KeySize, "key size"); err != nil {
			return nil, err
		}
		return newPBKDF2Provider(cfg.SaltSize, cfg.Iterations, cfg.KeySize), nil
	case ARGON2ID:
		if err := validatePositiveInt(cfg.SaltSize, "salt size"); err != nil {
			return nil, err
		}
		if err := validatePositiveIntWithMax(cfg.Memory, maxUint32, "memory"); err != nil {
			return nil, err
		}
		if err := validatePositiveIntWithMax(cfg.Iterations, maxUint32, "iterations"); err != nil {
			return nil, err
		}
		if err := validatePositiveIntWithMax(cfg.Parallelism, maxUint8, "parallelism"); err != nil {
			return nil, err
		}
		if err := validatePositiveIntWithMax(cfg.KeySize, maxUint32, "key size"); err != nil {
			return nil, err
		}
		return newArgon2idProvider(cfg.SaltSize, cfg.Memory, cfg.Iterations, cfg.Parallelism, cfg.KeySize), nil
	default:
		return nil, fmt.Errorf("unsupported hash algorithm: %s", cfg.Algorithm)
	}
}

func newSHA256Provider(saltSize int) *sha256HashProvider {
	return &sha256HashProvider{SaltSize: saltSize}
}

func (a *sha256HashProvider) Generate(credentialValue []byte) (Credential, error) {
	credSalt, err := generateSalt(a.SaltSize)
	if err != nil {
		return Credential{}, err
	}
	credentialWithSalt := append([]byte(nil), credentialValue...)
	credentialWithSalt = append(credentialWithSalt, credSalt...)
	h := sha256.Sum256(credentialWithSalt)
	return Credential{
		Algorithm: SHA256,
		Hash:      hex.EncodeToString(h[:]),
		Parameters: CredParameters{
			Salt: hex.EncodeToString(credSalt),
		},
	}, nil
}

func (a *sha256HashProvider) Verify(credentialValueToVerify []byte, referenceCredential Credential) (bool, error) {
	if err := validateCredentialAlgorithm(referenceCredential, SHA256); err != nil {
		return false, err
	}
	saltBytes, err := decodeSalt(referenceCredential.Parameters.Salt)
	if err != nil {
		return false, err
	}
	credentialWithSalt := append([]byte(nil), credentialValueToVerify...)
	credentialWithSalt = append(credentialWithSalt, saltBytes...)
	hashedData := sha256.Sum256(credentialWithSalt)
	referenceHash, err := hex.DecodeString(referenceCredential.Hash)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(hashedData[:], referenceHash) == 1, nil
}

func newPBKDF2Provider(saltSize, iterations, keySize int) *pbkdf2HashProvider {
	return &pbkdf2HashProvider{
		SaltSize:   saltSize,
		Iterations: iterations,
		KeySize:    keySize,
	}
}

func (a *pbkdf2HashProvider) Generate(credentialValue []byte) (Credential, error) {
	credSalt, err := generateSalt(a.SaltSize)
	if err != nil {
		return Credential{}, err
	}
	h, err := pbkdf2.Key(sha256.New, string(credentialValue), credSalt, a.Iterations, a.KeySize)
	if err != nil {
		return Credential{}, err
	}
	return Credential{
		Algorithm: PBKDF2,
		Hash:      hex.EncodeToString(h),
		Parameters: CredParameters{
			Iterations: a.Iterations,
			KeySize:    a.KeySize,
			Salt:       hex.EncodeToString(credSalt),
		},
	}, nil
}

func (a *pbkdf2HashProvider) Verify(credentialValueToVerify []byte, referenceCredential Credential) (bool, error) {
	if err := validateCredentialAlgorithm(referenceCredential, PBKDF2); err != nil {
		return false, err
	}
	iterations, err := requirePositiveInt(referenceCredential.Parameters.Iterations, "iterations")
	if err != nil {
		return false, err
	}
	keySize, err := requirePositiveInt(referenceCredential.Parameters.KeySize, "key size")
	if err != nil {
		return false, err
	}
	saltBytes, err := decodeSalt(referenceCredential.Parameters.Salt)
	if err != nil {
		return false, err
	}
	h, err := pbkdf2.Key(sha256.New, string(credentialValueToVerify), saltBytes, iterations, keySize)
	if err != nil {
		return false, err
	}
	referenceHash, err := hex.DecodeString(referenceCredential.Hash)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(h, referenceHash) == 1, nil
}

func newArgon2idProvider(saltSize, memory, iterations, parallelism, keySize int) *argon2idHashProvider {
	return &argon2idHashProvider{
		SaltSize:    saltSize,
		Memory:      memory,
		Iterations:  iterations,
		Parallelism: parallelism,
		KeySize:     keySize,
	}
}

func (a *argon2idHashProvider) Generate(credentialValue []byte) (Credential, error) {
	credSalt, err := generateSalt(a.SaltSize)
	if err != nil {
		return Credential{}, err
	}
	//nolint:gosec // G115 - Conversion is safe
	h := argon2.IDKey(
		credentialValue,
		credSalt,
		uint32(a.Iterations),
		uint32(a.Memory),
		uint8(a.Parallelism),
		uint32(a.KeySize),
	)
	return Credential{
		Algorithm: ARGON2ID,
		Hash:      hex.EncodeToString(h),
		Parameters: CredParameters{
			Memory:      a.Memory,
			Iterations:  a.Iterations,
			Parallelism: a.Parallelism,
			KeySize:     a.KeySize,
			Salt:        hex.EncodeToString(credSalt),
		},
	}, nil
}

func (a *argon2idHashProvider) Verify(credentialValueToVerify []byte, referenceCredential Credential) (bool, error) {
	if err := validateCredentialAlgorithm(referenceCredential, ARGON2ID); err != nil {
		return false, err
	}
	memory, err := requirePositiveIntWithMax(referenceCredential.Parameters.Memory, maxUint32, "memory")
	if err != nil {
		return false, err
	}
	iterations, err := requirePositiveIntWithMax(referenceCredential.Parameters.Iterations, maxUint32, "iterations")
	if err != nil {
		return false, err
	}
	parallelism, err := requirePositiveIntWithMax(referenceCredential.Parameters.Parallelism, maxUint8, "parallelism")
	if err != nil {
		return false, err
	}
	keySize, err := requirePositiveIntWithMax(referenceCredential.Parameters.KeySize, maxUint32, "key size")
	if err != nil {
		return false, err
	}
	saltBytes, err := decodeSalt(referenceCredential.Parameters.Salt)
	if err != nil {
		return false, err
	}
	//nolint:gosec // G115 - Conversion is safe
	h := argon2.IDKey(
		credentialValueToVerify,
		saltBytes,
		uint32(iterations),
		uint32(memory),
		uint8(parallelism),
		uint32(keySize),
	)
	referenceHash, err := hex.DecodeString(referenceCredential.Hash)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(h, referenceHash) == 1, nil
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

func validateCredentialAlgorithm(referenceCredential Credential, expected CredAlgorithm) error {
	if referenceCredential.Algorithm != expected {
		return fmt.Errorf("credential algorithm mismatch: expected %s", expected)
	}
	return nil
}

func validatePositiveInt(value int, name string) error {
	_, err := requirePositiveInt(value, name)
	return err
}

func validatePositiveIntWithMax(value, maxValue int, name string) error {
	_, err := requirePositiveIntWithMax(value, maxValue, name)
	return err
}

func requirePositiveInt(value int, name string) (int, error) {
	if value <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return value, nil
}

func requirePositiveIntWithMax(value, maxValue int, name string) (int, error) {
	normalized, err := requirePositiveInt(value, name)
	if err != nil {
		return 0, err
	}
	if normalized > maxValue {
		return 0, fmt.Errorf("%s exceeds maximum supported value", name)
	}
	return normalized, nil
}
