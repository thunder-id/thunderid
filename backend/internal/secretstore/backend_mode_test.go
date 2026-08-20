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

package secretstore

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestFileModeBuildsAFileBackend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")

	backend, err := NewBackend(Config{Mode: ModeFile, FilePath: path})

	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	if backend == nil {
		t.Fatal("expected a backend")
	}
	if !strings.Contains(backend.Name(), path) {
		t.Fatalf("expected the file backend, got %q", backend.Name())
	}
}

func TestFileModeRequiresAPath(t *testing.T) {
	if _, err := NewBackend(Config{Mode: ModeFile}); err == nil {
		t.Fatal("expected file mode with no path to be refused")
	}
}

// The modes that keep no store here build no backend, and that is not an error: a deployment reading
// from the standalone provider, or holding no secrets at all, is a valid deployment.
func TestTheModesThatKeepNoStoreHereBuildNoBackend(t *testing.T) {
	for _, mode := range []Mode{"", ModeService} {
		backend, err := NewBackend(Config{Mode: mode})
		if err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
		if backend != nil {
			t.Fatalf("mode %q should keep no store here, got %q", mode, backend.Name())
		}
	}
}

// A mistyped mode must not fall through to a default. A deployment that asked for a key vault and
// silently got a local file would be writing secrets somewhere nothing else can read them.
func TestAnUnknownModeIsRefusedAndSaysWhatIsExpected(t *testing.T) {
	_, err := NewBackend(Config{Mode: Mode("vault")})

	if err == nil {
		t.Fatal("expected an unknown mode to be refused")
	}
	for _, expected := range []string{string(ModeFile), string(ModeKV), string(ModeService)} {
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("expected the error to list %q, got %v", expected, err)
		}
	}
}

// OpenBao is the implemented vault, so it has to be registered for kv mode to work at all.
func TestOpenBaoIsRegistered(t *testing.T) {
	types := RegisteredKVTypes()

	for _, kind := range types {
		if kind == KVTypeOpenBao {
			return
		}
	}
	t.Fatalf("expected %q to be registered, got %v", KVTypeOpenBao, types)
}

// A vault with no implementation fails at startup rather than being ignored. Coming up regardless
// would leave a deployment believing it has secrets it cannot reach.
func TestAnUnimplementedKeyVaultIsRefusedAndListsWhatIsSupported(t *testing.T) {
	_, err := NewBackend(Config{Mode: ModeKV, KV: KVConfig{Type: "azure", Address: "https://x", Token: "t"}})

	if err == nil {
		t.Fatal("expected an unimplemented key vault to be refused")
	}
	if !strings.Contains(err.Error(), KVTypeOpenBao) {
		t.Fatalf("expected the error to list what is supported, got %v", err)
	}
}

func TestKVModeRequiresAType(t *testing.T) {
	_, err := NewBackend(Config{Mode: ModeKV, KV: KVConfig{Address: "https://x"}})

	if err == nil {
		t.Fatal("expected kv mode with no type to be refused")
	}
	if !strings.Contains(err.Error(), "kv.type") {
		t.Fatalf("expected the error to name the missing setting, got %v", err)
	}
}

// A container is handed its vault credential at startup rather than carrying it in an image, so the
// token comes from the environment when configuration leaves it empty.
func TestTheKeyVaultTokenIsTakenFromTheEnvironment(t *testing.T) {
	t.Setenv(EnvKVToken, "the-injected-token")

	backend, err := NewBackend(Config{
		Mode: ModeKV,
		KV:   KVConfig{Type: KVTypeOpenBao, Address: "https://vault:8200"},
	})

	if err != nil {
		t.Fatalf("expected the environment to supply the token, got %v", err)
	}
	if backend == nil {
		t.Fatal("expected a backend")
	}
}

// The injected credential wins: a token an image happened to ship must not override the one the
// deployment actually handed the container.
func TestTheEnvironmentTokenWinsOverAConfiguredOne(t *testing.T) {
	t.Setenv(EnvKVToken, "the-injected-token")

	backend, err := NewBackend(Config{
		Mode: ModeKV,
		KV:   KVConfig{Type: KVTypeOpenBao, Address: "https://vault:8200", Token: "the-baked-in-token"},
	})

	if err != nil {
		t.Fatalf("new backend: %v", err)
	}
	openBao, ok := backend.(*openBaoBackend)
	if !ok {
		t.Fatalf("expected an OpenBao backend, got %T", backend)
	}
	if openBao.token != "the-injected-token" {
		t.Fatalf("expected the injected token to win, got %q", openBao.token)
	}
}

// With no token from either source the deployment cannot reach its vault. It says so at startup
// rather than coming up and appearing to hold no secrets, and names where the token should come from.
func TestAMissingTokenNamesTheEnvironmentVariable(t *testing.T) {
	t.Setenv(EnvKVToken, "")

	_, err := NewBackend(Config{
		Mode: ModeKV,
		KV:   KVConfig{Type: KVTypeOpenBao, Address: "https://vault:8200"},
	})

	if err == nil {
		t.Fatal("expected a vault with no token to be refused")
	}
	if !strings.Contains(err.Error(), EnvKVToken) {
		t.Fatalf("expected the error to name %s, got %v", EnvKVToken, err)
	}
}

// The type is what an operator writes in configuration, so it must not be case sensitive.
func TestTheKeyVaultTypeIsCaseInsensitive(t *testing.T) {
	backend, err := NewBackend(Config{
		Mode: ModeKV,
		KV:   KVConfig{Type: "OpenBao", Address: "https://vault:8200", Token: "t"},
	})

	if err != nil {
		t.Fatalf("expected the type to be matched regardless of case, got %v", err)
	}
	if backend == nil {
		t.Fatal("expected a backend")
	}
}
