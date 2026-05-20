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

package hash

import (
	"crypto/pbkdf2"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/argon2"

	"github.com/thunder-id/thunderid/internal/system/config"
)

const (
	defaultSaltSize            = 16
	defaultPBKDF2Iterations    = 600000
	defaultPBKDF2KeySize       = 32
	defaultArgon2idMemory      = 19456 // 19 MB
	defaultArgon2idIterations  = 2
	defaultArgon2idParallelism = 1
	defaultArgon2idKeySize     = 32
)

type HashServiceTestSuite struct {
	suite.Suite
	input []byte
}

func sha256Hex(input string, saltHex string) string {
	saltBytes, err := hex.DecodeString(saltHex)
	if err != nil {
		panic(err)
	}
	data := append([]byte(input), saltBytes...)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func pbkdf2Hex(input string, saltHex string, iterations, keySize int) string {
	saltBytes, err := hex.DecodeString(saltHex)
	if err != nil {
		panic(err)
	}
	hash, err := pbkdf2.Key(sha256.New, input, saltBytes, iterations, keySize)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(hash)
}

func argon2idHex(input string, saltHex string, iterations, memory uint32, parallelism uint8, keySize uint32) string {
	saltBytes, err := hex.DecodeString(saltHex)
	if err != nil {
		panic(err)
	}
	hash := argon2.IDKey(
		[]byte(input),
		saltBytes,
		iterations,
		memory,
		parallelism,
		keySize,
	)
	return hex.EncodeToString(hash)
}

func TestHashServiceSuite(t *testing.T) {
	suite.Run(t, new(HashServiceTestSuite))
}

func (suite *HashServiceTestSuite) SetupSuite() {
	suite.input = []byte("secretPassword123")
}

func (suite *HashServiceTestSuite) TestGenerateSha256() {
	// Set runtime config to SHA256
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: defaultSaltSize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  defaultSaltSize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)

	assert.NoError(suite.T(), err, "Error should be nil when generating hash")
	assert.Equal(suite.T(), SHA256, cred.Algorithm, "Algorithm should be SHA256")
	assert.NotEmpty(suite.T(), cred.Hash, "Hash should not be empty")
}

func (suite *HashServiceTestSuite) TestSHA256HashWithCustomSaltSize() {
	customSaltSize := 32

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: customSaltSize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  customSaltSize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")
	assert.Equal(suite.T(), SHA256, cred.Algorithm, "Algorithm should be SHA256")
	assert.NotEmpty(suite.T(), cred.Hash, "Hash should not be empty")
	assert.NotEmpty(suite.T(), cred.Parameters.Salt, "Salt should not be empty")

	expectedSaltLength := customSaltSize * 2 // hex encoding doubles the length
	assert.Equal(suite.T(), expectedSaltLength, len(cred.Parameters.Salt),
		"Salt should be hex encoded with expected length")

	ok, err := hashService.Verify(suite.input, cred)
	assert.NoError(suite.T(), err, "Error should be nil when verifying hash")
	assert.True(suite.T(), ok, "Hash verification should succeed for the same input with custom salt size")
}

func (suite *HashServiceTestSuite) TestVerifySha256() {
	testCases := []struct {
		name    string
		input   string
		saltHex string
	}{
		{
			name:    "EmptyStringWithSalt",
			input:   "",
			saltHex: "12f4576d7432bd8020db7202b6492a37",
		},
		{
			name:    "NormalStringWithSalt",
			input:   "password",
			saltHex: "12f4576d7432bd8020db7202b6492a37",
		},
	}

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: defaultSaltSize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  defaultSaltSize,
	})
	require.NoError(suite.T(), err)

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			expected := Credential{
				Algorithm: SHA256,
				Hash:      sha256Hex(tc.input, tc.saltHex),
				Parameters: CredParameters{
					Salt: tc.saltHex,
				},
			}
			ok, err := hashService.Verify([]byte(tc.input), expected)
			assert.NoError(t, err, "Error should be nil when verifying hash")
			assert.True(t, ok)
		})
	}
}

func (suite *HashServiceTestSuite) TestVerifySha256_Failure() {
	testCases := []struct {
		name     string
		input    string
		expected Credential
		error    bool
	}{
		{
			name:  "IncorrectHash",
			input: "password",
			expected: Credential{
				Algorithm: SHA256,
				Hash:      "0000000000000000000000000000000000000000000000000000000000000000",
				Parameters: CredParameters{
					Salt: "12f4576d7432bd8020db7202b6492a37",
				},
			},
			error: false,
		},
		{
			name:  "IncorrectSalt",
			input: "password",
			expected: Credential{
				Algorithm: SHA256,
				Hash:      sha256Hex("password", "12f4576d7432bd8020db7202b6492a37"),
				Parameters: CredParameters{
					Salt: "incorrectsalt",
				},
			},
			error: true,
		},
		{
			name:  "MissingSalt",
			input: "password",
			expected: Credential{
				Algorithm: SHA256,
				Hash:      sha256Hex("password", "12f4576d7432bd8020db7202b6492a37"),
				Parameters: CredParameters{
					Salt: "",
				},
			},
			error: true,
		},
	}

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: defaultSaltSize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  defaultSaltSize,
	})
	require.NoError(suite.T(), err)

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ok, err := hashService.Verify([]byte(tc.input), tc.expected)
			assert.False(t, ok)
			if !tc.error {
				assert.NoError(t, err, "Error should be nil when verifying hash")
			} else {
				assert.Error(t, err, "Error should not be nil when verifying hash with invalid parameters")
			}
		})
	}
}

func (suite *HashServiceTestSuite) TestSha256HashAndVerify() {
	// Set runtime config to SHA256
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(SHA256),
				SHA256: config.SHA256Config{
					SaltSize: 16,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm: SHA256,
		SaltSize:  16,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")

	ok, err := hashService.Verify(suite.input, cred)
	assert.NoError(suite.T(), err, "Error should be nil when verifying hash")
	assert.True(suite.T(), ok, "Hash verification should succeed for the same input")
}

func (suite *HashServiceTestSuite) TestGeneratePBKDF2() {
	// Set runtime config to PBKDF2
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   defaultSaltSize,
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")
	assert.Equal(suite.T(), PBKDF2, cred.Algorithm, "Algorithm should be PBKDF2")
	assert.NotEmpty(suite.T(), cred.Hash, "Hash should not be empty")
}

func (suite *HashServiceTestSuite) TestPBKDF2HashWithCustomParameters() {
	customIterations := 100000
	customKeySize := 64
	customSaltSize := 32

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   customSaltSize,
					Iterations: customIterations,
					KeySize:    customKeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   customSaltSize,
		Iterations: customIterations,
		KeySize:    customKeySize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")
	assert.Equal(suite.T(), PBKDF2, cred.Algorithm, "Algorithm should be PBKDF2")
	assert.NotEmpty(suite.T(), cred.Hash, "Hash should not be empty")
	assert.NotEmpty(suite.T(), cred.Parameters.Salt, "Salt should not be empty")
	assert.Equal(suite.T(), customIterations, cred.Parameters.Iterations,
		"Credential should contain configured custom iterations")
	assert.Equal(suite.T(), customKeySize, cred.Parameters.KeySize,
		"Credential should contain configured custom key size")

	expectedSaltLength := customSaltSize * 2 // hex encoding doubles the length
	assert.Equal(suite.T(), expectedSaltLength, len(cred.Parameters.Salt),
		"Salt should be hex encoded with expected length")

	expectedHashLength := customKeySize * 2 // hex encoding doubles the length
	assert.Equal(suite.T(), expectedHashLength, len(cred.Hash),
		"Hash length should match configured key size")

	ok, err := hashService.Verify(suite.input, cred)
	assert.NoError(suite.T(), err, "Error should be nil when verifying hash")
	assert.True(suite.T(), ok, "Hash verification should succeed for the same input with custom parameters")
}

func (suite *HashServiceTestSuite) TestGeneratePBKDF2_Failure() {
	// Set runtime config to PBKDF2 with invalid parameters
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   defaultSaltSize,
					Iterations: defaultPBKDF2Iterations,
					KeySize:    -1,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	_, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    -1,
	})
	assert.Error(suite.T(), err, "Error should not be nil when initializing hash service with invalid parameters")
}

func (suite *HashServiceTestSuite) TestVerifyBKDF2() {
	testCases := []struct {
		name    string
		input   string
		saltHex string
		iter    int
		keySize int
	}{
		{
			name:    "EmptyStringWithSalt",
			input:   "",
			saltHex: "36d2dde7dfbafe8e04ea49450f659b1c",
			iter:    defaultPBKDF2Iterations,
			keySize: defaultPBKDF2KeySize,
		},
		{
			name:    "NormalStringWithSalt",
			input:   "password",
			saltHex: "36d2dde7dfbafe8e04ea49450f659b1c",
			iter:    defaultPBKDF2Iterations,
			keySize: defaultPBKDF2KeySize,
		},
	}

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   defaultSaltSize,
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			expected := Credential{
				Algorithm: PBKDF2,
				Hash:      pbkdf2Hex(tc.input, tc.saltHex, tc.iter, tc.keySize),
				Parameters: CredParameters{
					Salt:       tc.saltHex,
					Iterations: tc.iter,
					KeySize:    tc.keySize,
				},
			}
			ok, err := hashService.Verify([]byte(tc.input), expected)
			assert.NoError(t, err, "Error should be nil when verifying hash")
			assert.True(t, ok)
		})
	}
}

func (suite *HashServiceTestSuite) TestVerifyPBKDF2_Failure() {
	testCases := []struct {
		name     string
		input    string
		expected Credential
		error    bool
	}{
		{
			name:  "IncorrectHash",
			input: "password",
			expected: Credential{
				Algorithm: PBKDF2,
				Hash:      "0000000000000000000000000000000000000000000000000000000000000000",
				Parameters: CredParameters{
					Salt:       "36d2dde7dfbafe8e04ea49450f659b1c",
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
			error: false,
		},
		{
			name:  "IncorrectSalt",
			input: "password",
			expected: Credential{
				Algorithm: PBKDF2,
				Hash: pbkdf2Hex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultPBKDF2Iterations,
					defaultPBKDF2KeySize,
				),
				Parameters: CredParameters{
					Salt:       "incorrectsalt",
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
			error: true,
		},
		{
			name:  "IncorrectParameters",
			input: "password",
			expected: Credential{
				Algorithm: PBKDF2,
				Hash: pbkdf2Hex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultPBKDF2Iterations,
					defaultPBKDF2KeySize,
				),
				Parameters: CredParameters{
					Salt:       "36d2dde7dfbafe8e04ea49450f659b1c",
					Iterations: -1,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
			error: true,
		},
		{
			name:  "MissingSalt",
			input: "password",
			expected: Credential{
				Algorithm: PBKDF2,
				Hash: pbkdf2Hex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultPBKDF2Iterations,
					defaultPBKDF2KeySize,
				),
				Parameters: CredParameters{
					Salt:       "",
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
			error: true,
		},
	}

	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   defaultSaltSize,
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ok, err := hashService.Verify([]byte(tc.input), tc.expected)
			if !tc.error {
				assert.NoError(t, err, "Error should be nil when verifying hash")
			} else {
				assert.Error(t, err, "Error should not be nil when verifying hash with invalid parameters")
			}
			assert.False(t, ok)
		})
	}
}

func (suite *HashServiceTestSuite) TestPBKDF2HashWithAndVerify() {
	// Set runtime config to PBKDF2
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(PBKDF2),
				PBKDF2: config.PBKDF2Config{
					SaltSize:   defaultSaltSize,
					Iterations: defaultPBKDF2Iterations,
					KeySize:    defaultPBKDF2KeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")

	ok, err := hashService.Verify(suite.input, cred)
	assert.NoError(suite.T(), err, "Error should be nil when verifying hash")
	assert.True(suite.T(), ok, "Hash verification should succeed for the same input")
}

func TestArgon2idHex(t *testing.T) {
	testCases := []struct {
		name        string
		input       string
		saltHex     string
		iterations  uint32
		memory      uint32
		parallelism uint8
		keySize     uint32
	}{
		{
			name:        "DefaultParameters",
			input:       "password",
			saltHex:     "36d2dde7dfbafe8e04ea49450f659b1c",
			iterations:  defaultArgon2idIterations,
			memory:      defaultArgon2idMemory,
			parallelism: defaultArgon2idParallelism,
			keySize:     defaultArgon2idKeySize,
		},
		{
			name:        "HigherIterations",
			input:       "password",
			saltHex:     "36d2dde7dfbafe8e04ea49450f659b1c",
			iterations:  4,
			memory:      defaultArgon2idMemory,
			parallelism: defaultArgon2idParallelism,
			keySize:     defaultArgon2idKeySize,
		},
		{
			name:        "HigherMemory",
			input:       "password",
			saltHex:     "36d2dde7dfbafe8e04ea49450f659b1c",
			iterations:  defaultArgon2idIterations,
			memory:      65536,
			parallelism: defaultArgon2idParallelism,
			keySize:     defaultArgon2idKeySize,
		},
		{
			name:        "LargerKeySize",
			input:       "password",
			saltHex:     "36d2dde7dfbafe8e04ea49450f659b1c",
			iterations:  defaultArgon2idIterations,
			memory:      defaultArgon2idMemory,
			parallelism: defaultArgon2idParallelism,
			keySize:     64,
		},
		{
			name:        "EmptyInput",
			input:       "",
			saltHex:     "36d2dde7dfbafe8e04ea49450f659b1c",
			iterations:  defaultArgon2idIterations,
			memory:      defaultArgon2idMemory,
			parallelism: defaultArgon2idParallelism,
			keySize:     defaultArgon2idKeySize,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := argon2idHex(
				tc.input,
				tc.saltHex,
				tc.iterations,
				tc.memory,
				tc.parallelism,
				tc.keySize,
			)

			assert.NotEmpty(t, hash, "Hash should not be empty")

			hashBytes, err := hex.DecodeString(hash)
			assert.NoError(t, err, "Hash should be valid hex")
			assert.Equal(t, int(tc.keySize), len(hashBytes), "Hash length should match keySize")
		})
	}
}

func TestArgon2idHexConsistency(t *testing.T) {
	input := "password"
	saltHex := "36d2dde7dfbafe8e04ea49450f659b1c"
	iterations := uint32(2)
	memory := uint32(19456)
	parallelism := uint8(1)
	keySize := uint32(32)

	hash1 := argon2idHex(input, saltHex, iterations, memory, parallelism, keySize)
	hash2 := argon2idHex(input, saltHex, iterations, memory, parallelism, keySize)

	assert.Equal(t, hash1, hash2, "Same inputs should produce same hash")
}

func TestArgon2idHexInvalidSaltHex(t *testing.T) {
	invalidSalts := []string{
		"zzzz",
		"36d2dde7dfbafe8e04ea49450f659b1",
		"36d2dde7dfbafe8e04ea49450f659b1czz",
	}

	for _, saltHex := range invalidSalts {
		t.Run(saltHex, func(t *testing.T) {
			require.Panics(t, func() {
				argon2idHex("password", saltHex, 2, 19456, 1, 32)
			}, "Invalid salt hex should cause panic")
		})
	}
}

func (suite *HashServiceTestSuite) TestGenerateArgon2id() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(ARGON2ID),
				Argon2ID: config.Argon2IDConfig{
					SaltSize:    defaultSaltSize,
					Memory:      defaultArgon2idMemory,
					Iterations:  defaultArgon2idIterations,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:   ARGON2ID,
		SaltSize:    defaultSaltSize,
		Memory:      defaultArgon2idMemory,
		Iterations:  defaultArgon2idIterations,
		Parallelism: defaultArgon2idParallelism,
		KeySize:     defaultArgon2idKeySize,
	})
	require.NoError(suite.T(), err)

	cred, err := hashService.Generate(suite.input)
	assert.NoError(suite.T(), err, "Error should be nil when generating hash")
	assert.Equal(suite.T(), ARGON2ID, cred.Algorithm, "Algorithm should be Argon2id")
	assert.NotEmpty(suite.T(), cred.Hash, "Hash should not be empty")
}

func (suite *HashServiceTestSuite) TestVerifyArgon2id() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(ARGON2ID),
				Argon2ID: config.Argon2IDConfig{
					SaltSize:    defaultSaltSize,
					Memory:      defaultArgon2idMemory,
					Iterations:  defaultArgon2idIterations,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:   ARGON2ID,
		SaltSize:    defaultSaltSize,
		Memory:      defaultArgon2idMemory,
		Iterations:  defaultArgon2idIterations,
		Parallelism: defaultArgon2idParallelism,
		KeySize:     defaultArgon2idKeySize,
	})
	require.NoError(suite.T(), err)

	expected := Credential{
		Algorithm: ARGON2ID,
		Hash: argon2idHex(
			"password",
			"36d2dde7dfbafe8e04ea49450f659b1c",
			defaultArgon2idIterations,
			defaultArgon2idMemory,
			defaultArgon2idParallelism,
			defaultArgon2idKeySize,
		),
		Parameters: CredParameters{
			Salt:        "36d2dde7dfbafe8e04ea49450f659b1c",
			Iterations:  defaultArgon2idIterations,
			Memory:      defaultArgon2idMemory,
			Parallelism: defaultArgon2idParallelism,
			KeySize:     defaultArgon2idKeySize,
		},
	}

	ok, err := hashService.Verify([]byte("password"), expected)
	assert.NoError(suite.T(), err, "Error should be nil when verifying hash")
	assert.True(suite.T(), ok)
}

func (suite *HashServiceTestSuite) TestVerifyArgon2id_Failure() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(ARGON2ID),
				Argon2ID: config.Argon2IDConfig{
					SaltSize:    defaultSaltSize,
					Memory:      defaultArgon2idMemory,
					Iterations:  defaultArgon2idIterations,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	hashService, err := Initialize(HashConfig{
		Algorithm:   ARGON2ID,
		SaltSize:    defaultSaltSize,
		Memory:      defaultArgon2idMemory,
		Iterations:  defaultArgon2idIterations,
		Parallelism: defaultArgon2idParallelism,
		KeySize:     defaultArgon2idKeySize,
	})
	require.NoError(suite.T(), err)

	testCases := []struct {
		name     string
		input    string
		expected Credential
		error    bool
	}{
		{
			name:  "IncorrectHash",
			input: "password",
			expected: Credential{
				Algorithm: ARGON2ID,
				Hash:      "0000000000000000000000000000000000000000000000000000000000000000",
				Parameters: CredParameters{
					Salt:        "36d2dde7dfbafe8e04ea49450f659b1c",
					Iterations:  defaultArgon2idIterations,
					Memory:      defaultArgon2idMemory,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
			error: false,
		},
		{
			name:  "IncorrectSalt",
			input: "password",
			expected: Credential{
				Algorithm: ARGON2ID,
				Hash: argon2idHex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultArgon2idIterations,
					defaultArgon2idMemory,
					defaultArgon2idParallelism,
					defaultArgon2idKeySize,
				),
				Parameters: CredParameters{
					Salt:        "incorrectsalt",
					Iterations:  defaultArgon2idIterations,
					Memory:      defaultArgon2idMemory,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
			error: true,
		},
		{
			name:  "IncorrectParameters",
			input: "password",
			expected: Credential{
				Algorithm: ARGON2ID,
				Hash: argon2idHex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultArgon2idIterations,
					defaultArgon2idMemory,
					defaultArgon2idParallelism,
					defaultArgon2idKeySize,
				),
				Parameters: CredParameters{
					Salt:        "36d2dde7dfbafe8e04ea49450f659b1c",
					Iterations:  -1,
					Memory:      defaultArgon2idMemory,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
			error: true,
		},
		{
			name:  "MissingSalt",
			input: "password",
			expected: Credential{
				Algorithm: ARGON2ID,
				Hash: argon2idHex(
					"password",
					"36d2dde7dfbafe8e04ea49450f659b1c",
					defaultArgon2idIterations,
					defaultArgon2idMemory,
					defaultArgon2idParallelism,
					defaultArgon2idKeySize,
				),
				Parameters: CredParameters{
					Salt:        "",
					Iterations:  defaultArgon2idIterations,
					Memory:      defaultArgon2idMemory,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
			error: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ok, err := hashService.Verify([]byte(tc.input), tc.expected)
			if !tc.error {
				assert.NoError(t, err, "Error should be nil when verifying hash")
			} else {
				assert.Error(t, err, "Error should not be nil when verifying hash with invalid parameters")
			}
			assert.False(t, ok)
		})
	}
}

func (suite *HashServiceTestSuite) TestGenerateArgon2id_Failure() {
	testConfig := &config.Config{
		Crypto: config.CryptoConfig{
			PasswordHashing: config.PasswordHashingConfig{
				Algorithm: string(ARGON2ID),
				Argon2ID: config.Argon2IDConfig{
					SaltSize:    defaultSaltSize,
					Memory:      -1,
					Iterations:  defaultArgon2idIterations,
					Parallelism: defaultArgon2idParallelism,
					KeySize:     defaultArgon2idKeySize,
				},
			},
		},
	}
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("/test/thunderid/home", testConfig)

	_, err := Initialize(HashConfig{
		Algorithm:   ARGON2ID,
		SaltSize:    defaultSaltSize,
		Memory:      -1,
		Iterations:  defaultArgon2idIterations,
		Parallelism: defaultArgon2idParallelism,
		KeySize:     defaultArgon2idKeySize,
	})
	assert.Error(suite.T(), err,
		"Error should not be nil when initializing Argon2id hash service with invalid parameters")
}

func (suite *HashServiceTestSuite) TestUnsupportedAlgorithm_Failure() {
	_, err := newHashService(HashConfig{Algorithm: "UNSUPPORTED"})
	assert.Error(suite.T(), err, "Error should not be nil for unsupported algorithm")
}

func (suite *HashServiceTestSuite) TestGenerateSalt() {
	salt, err := generateSalt(defaultSaltSize)
	assert.NoError(suite.T(), err, "Error should be nil when generating salt")
	assert.NotEmpty(suite.T(), salt)
	assert.Equal(suite.T(), 16, len(salt), "Generated salt should be 16 bytes")
}

func (suite *HashServiceTestSuite) TestGenerateSaltUniqueness() {
	salt1, err := generateSalt(defaultSaltSize)
	assert.NoError(suite.T(), err, "Error should be nil when generating salt")
	salt2, err := generateSalt(defaultSaltSize)
	assert.NoError(suite.T(), err, "Error should be nil when generating salt")

	assert.NotEqual(suite.T(), salt1, salt2, "Generated salts should be different")
}

func (suite *HashServiceTestSuite) TestInitialize() {
	hashService, err := Initialize(HashConfig{Algorithm: SHA256, SaltSize: defaultSaltSize})
	assert.NoError(suite.T(), err, "Error should be nil when initializing hash service")
	assert.NotNil(suite.T(), hashService, "Hash service should not be nil")
}

func (suite *HashServiceTestSuite) TestInitialize_Failure() {
	_, err := Initialize(HashConfig{Algorithm: "UNSUPPORTED"})
	assert.Error(suite.T(), err, "Error should not be nil when initializing hash service with unsupported algorithm")
}

func (suite *HashServiceTestSuite) TestVerifyPBKDF2_InvalidKeySize() {
	hashService, err := newHashService(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	credential := Credential{
		Algorithm: PBKDF2,
		Hash: pbkdf2Hex(
			"password",
			"36d2dde7dfbafe8e04ea49450f659b1c",
			defaultPBKDF2Iterations,
			defaultPBKDF2KeySize,
		),
		Parameters: CredParameters{
			Salt:       "36d2dde7dfbafe8e04ea49450f659b1c",
			Iterations: defaultPBKDF2Iterations,
			KeySize:    -1,
		},
	}
	ok, err := hashService.Verify([]byte("password"), credential)
	assert.Error(suite.T(), err, "Error should not be nil when verifying with invalid key size")
	assert.False(suite.T(), ok)
}

func (suite *HashServiceTestSuite) TestVerifyArgon2id_InvalidMemory() {
	testCases := []struct {
		name string
		cfg  HashConfig
	}{
		{
			name: "UnsupportedAlgorithm",
			cfg:  HashConfig{Algorithm: "UNSUPPORTED"},
		},
		{
			name: "SHA256InvalidSaltSize",
			cfg:  HashConfig{Algorithm: SHA256, SaltSize: -1},
		},
		{
			name: "PBKDF2InvalidSaltSize",
			cfg: HashConfig{
				Algorithm: PBKDF2, SaltSize: -1, Iterations: defaultPBKDF2Iterations, KeySize: defaultPBKDF2KeySize,
			},
		},
		{
			name: "PBKDF2InvalidIterations",
			cfg: HashConfig{
				Algorithm: PBKDF2, SaltSize: defaultSaltSize, Iterations: -1, KeySize: defaultPBKDF2KeySize,
			},
		},
		{
			name: "PBKDF2InvalidKeySize",
			cfg: HashConfig{
				Algorithm: PBKDF2, SaltSize: defaultSaltSize, Iterations: defaultPBKDF2Iterations, KeySize: -1,
			},
		},
		{
			name: "Argon2idInvalidSaltSize",
			cfg: HashConfig{
				Algorithm: ARGON2ID, SaltSize: -1, Memory: defaultArgon2idMemory,
				Iterations: defaultArgon2idIterations, Parallelism: defaultArgon2idParallelism,
				KeySize: defaultArgon2idKeySize,
			},
		},
		{
			name: "Argon2idInvalidMemory",
			cfg: HashConfig{
				Algorithm: ARGON2ID, SaltSize: defaultSaltSize, Memory: -1,
				Iterations: defaultArgon2idIterations, Parallelism: defaultArgon2idParallelism,
				KeySize: defaultArgon2idKeySize,
			},
		},
		{
			name: "Argon2idInvalidParallelism",
			cfg: HashConfig{
				Algorithm: ARGON2ID, SaltSize: defaultSaltSize, Memory: defaultArgon2idMemory,
				Iterations: defaultArgon2idIterations, Parallelism: -1, KeySize: defaultArgon2idKeySize,
			},
		},
		{
			name: "Argon2idInvalidKeySize",
			cfg: HashConfig{
				Algorithm: ARGON2ID, SaltSize: defaultSaltSize, Memory: defaultArgon2idMemory,
				Iterations: defaultArgon2idIterations, Parallelism: defaultArgon2idParallelism,
				KeySize: -1,
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			_, err := Initialize(tc.cfg)
			assert.Error(t, err, "Error should not be nil for invalid config: %s", tc.name)
		})
	}
}

func (suite *HashServiceTestSuite) TestUnsupportedAlgorithmVerify_Failure() {
	hashService, err := Initialize(HashConfig{
		Algorithm:  PBKDF2,
		SaltSize:   defaultSaltSize,
		Iterations: defaultPBKDF2Iterations,
		KeySize:    defaultPBKDF2KeySize,
	})
	require.NoError(suite.T(), err)

	badCred := Credential{
		Algorithm: "UNSUPPORTED",
		Hash:      "0000000000000000000000000000000000000000000000000000000000000000",
		Parameters: CredParameters{
			Salt:       "36d2dde7dfbafe8e04ea49450f659b1c",
			Iterations: defaultPBKDF2Iterations,
			KeySize:    defaultPBKDF2KeySize,
		},
	}

	ok, err := hashService.Verify([]byte("password"), badCred)
	assert.Error(suite.T(), err)
	assert.False(suite.T(), ok)
}
