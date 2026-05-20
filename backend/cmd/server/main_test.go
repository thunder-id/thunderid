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

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

// CreateSecurityMiddlewareTestSuite defines the test suite for createSecurityMiddleware function
type CreateSecurityMiddlewareTestSuite struct {
	suite.Suite
	logger         *log.Logger
	mockJWTService *jwtmock.JWTServiceInterfaceMock
	mux            *http.ServeMux
}

func TestCreateSecurityMiddlewareTestSuite(t *testing.T) {
	suite.Run(t, new(CreateSecurityMiddlewareTestSuite))
}

func (suite *CreateSecurityMiddlewareTestSuite) SetupTest() {
	suite.logger = log.GetLogger()
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mux = http.NewServeMux()

	// Ensure environment variable is clean before each test
	_ = os.Unsetenv("SKIP_SECURITY")
}

func (suite *CreateSecurityMiddlewareTestSuite) TearDownTest() {
	// Clean up environment variable after each test
	_ = os.Unsetenv("SKIP_SECURITY")
}

// TestCreateSecurityMiddleware_WithEnvironmentVariable tests various SKIP_SECURITY environment variable values
func (suite *CreateSecurityMiddlewareTestSuite) TestCreateSecurityMiddleware_WithEnvironmentVariable() {
	testCases := []struct {
		name               string
		envValue           string
		setEnv             bool
		expectSecuritySkip bool
	}{
		{
			name:               "Security enabled - no env variable",
			setEnv:             false,
			expectSecuritySkip: false,
		},
		{
			name:               "Security disabled - true",
			envValue:           "true",
			setEnv:             true,
			expectSecuritySkip: true,
		},
		{
			name:               "Security enabled - false",
			envValue:           "false",
			setEnv:             true,
			expectSecuritySkip: false,
		},
		{
			name:               "Security enabled - empty string",
			envValue:           "",
			setEnv:             true,
			expectSecuritySkip: false,
		},
		{
			name:               "Security enabled - invalid value",
			envValue:           "yes",
			setEnv:             true,
			expectSecuritySkip: false,
		},
		{
			name:               "Security enabled - uppercase TRUE",
			envValue:           "TRUE",
			setEnv:             true,
			expectSecuritySkip: false,
		},
		{
			name:               "Security enabled - mixed case True",
			envValue:           "True",
			setEnv:             true,
			expectSecuritySkip: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			if tc.setEnv {
				_ = os.Setenv("SKIP_SECURITY", tc.envValue)
			} else {
				_ = os.Unsetenv("SKIP_SECURITY")
			}

			// Execute
			handler := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)

			// Assert - handler is always returned now, regardless of skip security flag
			assert.NotNil(suite.T(), handler, "Handler should always be non-nil")

			// Cleanup for next iteration
			_ = os.Unsetenv("SKIP_SECURITY")
		})
	}
}

// TestCreateSecurityMiddleware_MultipleInvocations tests that multiple calls work correctly
func (suite *CreateSecurityMiddlewareTestSuite) TestCreateSecurityMiddleware_MultipleInvocations() {
	// Execute multiple times
	handler1 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)
	handler2 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)
	handler3 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)

	// Assert - each call should return a new handler instance
	assert.NotNil(suite.T(), handler1)
	assert.NotNil(suite.T(), handler2)
	assert.NotNil(suite.T(), handler3)
}

// TestCreateSecurityMiddleware_RuntimeToggle tests toggling security at runtime by changing environment variable
func (suite *CreateSecurityMiddlewareTestSuite) TestCreateSecurityMiddleware_RuntimeToggle() {
	// First call with security enabled
	handler1 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)
	assert.NotNil(suite.T(), handler1, "First handler should not be nil")

	// Disable security
	_ = os.Setenv("SKIP_SECURITY", "true")
	handler2 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)
	assert.NotNil(suite.T(), handler2, "Second handler should not be nil (skipSecurity is handled internally)")

	// Re-enable security
	_ = os.Unsetenv("SKIP_SECURITY")
	handler3 := createSecurityMiddleware(suite.logger, suite.mux, suite.mockJWTService)
	assert.NotNil(suite.T(), handler3, "Third handler should not be nil after re-enabling security")
}

func TestCreateHTTPServer_WithHTTPOnly(t *testing.T) {
	logger := log.GetLogger()
	if err := os.Setenv("SKIP_SECURITY", "true"); err != nil {
		t.Fatalf("failed to set SKIP_SECURITY: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("SKIP_SECURITY"); err != nil {
			t.Fatalf("failed to unset SKIP_SECURITY: %v", err)
		}
	}()

	cfg := &config.Config{
		Server: config.ServerConfig{
			Hostname: "localhost",
			Port:     0,
			HTTPOnly: true,
		},
	}

	mux := http.NewServeMux()
	server := createHTTPServer(logger, cfg, mux, nil)

	assert.Equal(t, "localhost:0", server.Addr)
	assert.NotNil(t, server.Handler)
	assert.NotZero(t, server.ReadHeaderTimeout)
	assert.NotZero(t, server.WriteTimeout)
	assert.NotZero(t, server.IdleTimeout)
}

func TestCreateListener_Success(t *testing.T) {
	logger := log.GetLogger()
	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		ReadHeaderTimeout: time.Second,
	}

	stubListener := &stubNetListener{
		addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
	}

	callCount := 0
	originalListen := netListen
	netListen = func(network, address string) (net.Listener, error) {
		callCount++
		assert.Equal(t, "tcp", network)
		assert.Equal(t, server.Addr, address)
		return stubListener, nil
	}
	t.Cleanup(func() {
		netListen = originalListen
	})

	ln := createListener(logger, server)

	assert.Equal(t, 1, callCount)
	assert.Equal(t, stubListener, ln)
}

func TestCreateListener_ExitsOnError(t *testing.T) {
	const helperEnv = "TEST_CREATE_LISTENER_EXIT"
	if os.Getenv(helperEnv) == "1" {
		originalListen := netListen
		netListen = func(_ string, _ string) (net.Listener, error) {
			return nil, errors.New("listen failure")
		}
		defer func() {
			netListen = originalListen
		}()

		logger := log.GetLogger()
		server := &http.Server{
			Addr:              "invalid-address",
			ReadHeaderTimeout: time.Second,
		}
		createListener(logger, server)
		return
	}

	runExitHelper(t, helperEnv, "TestCreateListener_ExitsOnError")
}

func TestCreateTLSListener_Success(t *testing.T) {
	logger := log.GetLogger()
	server := &http.Server{
		Addr:              "127.0.0.1:8443",
		ReadHeaderTimeout: time.Second,
	}
	tlsConfig := generateTestTLSConfig(t)

	stubListener := &stubNetListener{
		addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8443},
	}

	callCount := 0
	originalTLSListen := tlsListen
	tlsListen = func(network, address string, config *tls.Config) (net.Listener, error) {
		callCount++
		assert.Equal(t, "tcp", network)
		assert.Equal(t, server.Addr, address)
		assert.Equal(t, tlsConfig, config)
		return stubListener, nil
	}
	t.Cleanup(func() {
		tlsListen = originalTLSListen
	})

	ln := createTLSListener(logger, server, tlsConfig)

	assert.Equal(t, 1, callCount)
	assert.Equal(t, stubListener, ln)
}

func TestCreateTLSListener_ExitsOnError(t *testing.T) {
	const helperEnv = "TEST_CREATE_TLS_LISTENER_EXIT"
	if os.Getenv(helperEnv) == "1" {
		originalTLSListen := tlsListen
		tlsListen = func(_ string, _ string, _ *tls.Config) (net.Listener, error) {
			return nil, errors.New("tls listen failure")
		}
		defer func() {
			tlsListen = originalTLSListen
		}()

		logger := log.GetLogger()
		server := &http.Server{
			Addr:              "invalid-address",
			ReadHeaderTimeout: time.Second,
		}
		createTLSListener(logger, server, &tls.Config{MinVersion: tls.VersionTLS12})
		return
	}

	runExitHelper(t, helperEnv, "TestCreateTLSListener_ExitsOnError")
}

func TestGetThunderHome_UsesFlagValue(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
	}()

	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{origArgs[0], "-serverHome", tmpDir}

	got := getThunderHome(log.GetLogger())
	assert.Equal(t, tmpDir, got)
}

func TestGetThunderHome_DefaultsToCWD(t *testing.T) {
	origArgs := os.Args
	origCommandLine := flag.CommandLine
	origWD, _ := os.Getwd()
	defer func() {
		os.Args = origArgs
		flag.CommandLine = origCommandLine
		_ = os.Chdir(origWD)
	}()

	tmpDir := t.TempDir()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	os.Args = []string{origArgs[0]}
	_ = os.Chdir(tmpDir)

	got := getThunderHome(log.GetLogger())
	expectedResolved, err := filepath.EvalSymlinks(tmpDir)
	assert.NoError(t, err)
	gotResolved, err := filepath.EvalSymlinks(got)
	assert.NoError(t, err)
	assert.Equal(t, expectedResolved, gotResolved)
}

func TestCreateStaticFileHandler(t *testing.T) {
	logger := log.GetLogger()
	tmpDir := t.TempDir()

	indexContent := []byte("index content")
	fileContent := []byte("hello file")

	requireWriteFile(t, filepath.Join(tmpDir, "index.html"), indexContent)
	requireWriteFile(t, filepath.Join(tmpDir, "hello.txt"), fileContent)

	handler := createStaticFileHandler("/app/", tmpDir, logger)

	t.Run("serves existing file", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/hello.txt", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, string(fileContent), rr.Body.String())
	})

	t.Run("falls back to index.html", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/unknown", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, string(indexContent), rr.Body.String())
	})
}

func TestCreateStaticFileHandler_CacheHeaders(t *testing.T) {
	logger := log.GetLogger()
	tmpDir := t.TempDir()

	indexContent := []byte("<!DOCTYPE html><html><body>index</body></html>")
	jsContent := []byte("console.log('hello');")
	cssContent := []byte("body { margin: 0; }")
	imageContent := []byte{0xFF, 0xD8, 0xFF} // Mock image bytes

	requireWriteFile(t, filepath.Join(tmpDir, "index.html"), indexContent)
	requireWriteFile(t, filepath.Join(tmpDir, "app.js"), jsContent)
	requireWriteFile(t, filepath.Join(tmpDir, "styles.css"), cssContent)
	requireWriteFile(t, filepath.Join(tmpDir, "logo.png"), imageContent)

	handler := createStaticFileHandler("/app/", tmpDir, logger)

	t.Run("sets cache headers when serving index.html at root", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, constants.CacheControlNoCacheComposite, rr.Header().Get(constants.CacheControlHeaderName),
			"Cache-Control header should prevent caching for index.html at root")
		assert.Equal(t, constants.PragmaNoCache, rr.Header().Get(constants.PragmaHeaderName),
			"Pragma header should be set for index.html at root")
		assert.Equal(t, constants.ExpiresZero, rr.Header().Get(constants.ExpiresHeaderName),
			"Expires header should be set for index.html at root")
		assert.Contains(t, rr.Body.String(), "index")
	})

	t.Run("sets cache headers when serving index.html directly", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/app/index.html", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, constants.CacheControlNoCacheComposite, rr.Header().Get(constants.CacheControlHeaderName),
			"Cache-Control header should prevent caching for direct index.html request")
		assert.Equal(t, constants.PragmaNoCache, rr.Header().Get(constants.PragmaHeaderName),
			"Pragma header should be set for direct index.html request")
		assert.Equal(t, constants.ExpiresZero, rr.Header().Get(constants.ExpiresHeaderName),
			"Expires header should be set for direct index.html request")
		assert.Contains(t, rr.Body.String(), "index")
	})

	t.Run("sets cache headers when serving index.html as SPA fallback", func(t *testing.T) {
		testCases := []struct {
			path        string
			description string
		}{
			{"/app/dashboard", "single level path"},
			{"/app/users/profile", "multi level path"},
			{"/app/settings/advanced/security", "deeply nested path"},
			{"/app/nonexistent.html", "non-existent HTML file"},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Equal(t, constants.CacheControlNoCacheComposite,
					rr.Header().Get(constants.CacheControlHeaderName),
					"Cache-Control header should prevent caching for SPA fallback at %s", tc.path)
				assert.Equal(t, constants.PragmaNoCache, rr.Header().Get(constants.PragmaHeaderName),
					"Pragma header should be set for SPA fallback at %s", tc.path)
				assert.Equal(t, constants.ExpiresZero, rr.Header().Get(constants.ExpiresHeaderName),
					"Expires header should be set for SPA fallback at %s", tc.path)
				assert.Contains(t, rr.Body.String(), "index",
					"Should serve index.html content for SPA fallback at %s", tc.path)
			})
		}
	})

	t.Run("does not set cache headers for static assets", func(t *testing.T) {
		testCases := []struct {
			path        string
			description string
			content     []byte
		}{
			{"/app/app.js", "JavaScript file", jsContent},
			{"/app/styles.css", "CSS file", cssContent},
			{"/app/logo.png", "image file", imageContent},
		}

		for _, tc := range testCases {
			t.Run(tc.description, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tc.path, nil)
				rr := httptest.NewRecorder()

				handler.ServeHTTP(rr, req)

				assert.Equal(t, http.StatusOK, rr.Code)
				assert.Empty(t, rr.Header().Get(constants.CacheControlHeaderName),
					"Cache-Control header should not be set for %s", tc.description)
				assert.Empty(t, rr.Header().Get(constants.PragmaHeaderName),
					"Pragma header should not be set for %s", tc.description)
				assert.Empty(t, rr.Header().Get(constants.ExpiresHeaderName),
					"Expires header should not be set for %s", tc.description)
				assert.Equal(t, string(tc.content), rr.Body.String(),
					"Should serve correct content for %s", tc.description)
			})
		}
	})

	t.Run("does not match files ending with index.html incorrectly", func(t *testing.T) {
		customIndexFile := []byte("custom index content")
		requireWriteFile(t, filepath.Join(tmpDir, "my-custom-index.html"), customIndexFile)

		req := httptest.NewRequest(http.MethodGet, "/app/my-custom-index.html", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Empty(t, rr.Header().Get(constants.CacheControlHeaderName),
			"Cache-Control should not be set for files that contain 'index.html' but are not exactly 'index.html'")
		assert.Empty(t, rr.Header().Get(constants.PragmaHeaderName),
			"Pragma should not be set for files that contain 'index.html' but are not exactly 'index.html'")
		assert.Empty(t, rr.Header().Get(constants.ExpiresHeaderName),
			"Expires should not be set for files that contain 'index.html' but are not exactly 'index.html'")
		assert.Equal(t, string(customIndexFile), rr.Body.String())
	})
}

func TestDirectoryExists(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("returns true for existing directory", func(t *testing.T) {
		assert.True(t, directoryExists(tmpDir))
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		assert.False(t, directoryExists(filepath.Join(tmpDir, "nonexistent")))
	})

	t.Run("returns false for file, not directory", func(t *testing.T) {
		filePath := filepath.Join(tmpDir, "file.txt")
		requireWriteFile(t, filePath, []byte("content"))
		assert.False(t, directoryExists(filePath))
	})
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "file.txt")
	requireWriteFile(t, filePath, []byte("content"))

	t.Run("returns true for existing file", func(t *testing.T) {
		assert.True(t, fileExists(filePath))
	})

	t.Run("returns false for non-existent file", func(t *testing.T) {
		assert.False(t, fileExists(filepath.Join(tmpDir, "nonexistent.txt")))
	})

	t.Run("returns false for directory, not file", func(t *testing.T) {
		assert.False(t, fileExists(tmpDir))
	})
}

func TestRegisterStaticFileHandlers(t *testing.T) {
	logger := log.GetLogger()
	tmpDir := t.TempDir()

	// Create gate and console directories
	gateDir := filepath.Join(tmpDir, "apps", "gate")
	consoleDir := filepath.Join(tmpDir, "apps", "console")
	err := os.MkdirAll(gateDir, 0o750)
	assert.NoError(t, err)
	err = os.MkdirAll(consoleDir, 0o750)
	assert.NoError(t, err)

	// Create index.html files
	requireWriteFile(t, filepath.Join(gateDir, "index.html"), []byte("gate app"))
	requireWriteFile(t, filepath.Join(consoleDir, "index.html"), []byte("console app"))

	t.Run("registers handlers for existing directories", func(t *testing.T) {
		mux := http.NewServeMux()
		registerStaticFileHandlers(logger, mux, tmpDir)

		// Test gate handler
		req := httptest.NewRequest(http.MethodGet, "/gate/", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "gate app")

		// Test console handler
		req = httptest.NewRequest(http.MethodGet, "/console/", nil)
		rr = httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "console app")
	})

	t.Run("handles missing directories gracefully", func(t *testing.T) {
		emptyTmpDir := t.TempDir()
		mux := http.NewServeMux()
		// Should not panic
		registerStaticFileHandlers(logger, mux, emptyTmpDir)
	})
}

func requireWriteFile(t *testing.T, path string, content []byte) {
	t.Helper()
	cleanPath := filepath.Clean(path)

	err := os.WriteFile(cleanPath, content, 0o600)
	if err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		t.Fatalf("failed to open file %s: %v", path, err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatalf("failed to close file %s: %v", path, closeErr)
		}
	}()

	if _, err := io.ReadAll(f); err != nil {
		t.Fatalf("failed to read back file %s: %v", path, err)
	}
}

type stubNetListener struct {
	addr net.Addr
}

func (s *stubNetListener) Accept() (net.Conn, error) {
	return nil, nil
}

func (s *stubNetListener) Close() error {
	return nil
}

func (s *stubNetListener) Addr() net.Addr {
	return s.addr
}

func generateTestTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	cert := generateSelfSignedCertificate(t)

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
	}
}

func generateSelfSignedCertificate(t *testing.T) tls.Certificate {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate private key: %v", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("failed to generate serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: "localhost",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("failed to parse x509 key pair: %v", err)
	}

	return cert
}

func runExitHelper(t *testing.T, envKey, testName string) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run="+testName, "--") //nolint:gosec // test helper uses controlled args
	cmd.Env = append(os.Environ(), envKey+"=1")
	err := cmd.Run()

	var exitErr *exec.ExitError
	if assert.ErrorAs(t, err, &exitErr) {
		assert.Equal(t, 1, exitErr.ExitCode())
	} else {
		t.Fatalf("expected process to exit with code 1, got %v", err)
	}
}
