//go:build integration

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	appsv1alpha1 "github.com/thunder-id/thunderid/tools/k8s-operator/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = appsv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	if dir := resolveEnvTestBinDir(); dir != "" {
		testEnv.BinaryAssetsDirectory = dir
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	// envtest.Environment.Stop() sends a process signal Windows doesn't support ("not
	// supported by windows"), on every run, regardless of whether teardown itself is fine —
	// treat that specific error as success instead of retrying it for a full minute and
	// failing the suite over an OS limitation rather than an actual problem.
	Eventually(func() error {
		err := testEnv.Stop()
		if err != nil && strings.Contains(err.Error(), "not supported by windows") {
			return nil
		}
		return err
	}, time.Minute, time.Second).Should(Succeed())

	// The "not supported by windows" error above means Stop() never got past its first
	// SIGTERM attempt (Go's os.Process.Signal only supports the special os.Kill case on
	// Windows, so it errors out immediately instead of falling through to its own SIGKILL
	// fallback) — etcd/kube-apiserver are left running as orphans. Finish the job ourselves,
	// matched by exact binary path so this only ever touches the processes this suite itself
	// started, never some unrelated etcd/kube-apiserver on the machine.
	killOrphanedEnvtestProcesses()
})

// killOrphanedEnvtestProcesses force-kills the etcd/kube-apiserver processes this suite started,
// working around Stop()'s inability to signal them on Windows (see the AfterSuite comment above).
// No-op on platforms where Stop() can actually terminate its own children.
func killOrphanedEnvtestProcesses() {
	if runtime.GOOS != "windows" {
		return
	}
	for _, path := range []string{testEnv.ControlPlane.Etcd.Path, testEnv.ControlPlane.APIServer.Path} {
		if path == "" {
			continue
		}
		// process.State stores the binary path without ".exe" (Windows only resolves that at
		// process-start time), but a running process's own reported ExecutablePath always has
		// it — append it here so the match actually lands.
		script := `Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -eq '` + path + `.exe' } | ` +
			`ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`
		_ = exec.Command("powershell", "-NoProfile", "-Command", script).Run()
	}
}

// envtestK8sVersion derives the same version the Makefile's ENVTEST_K8S_VERSION does (go.mod's
// k8s.io/api "v0.<minor>.x" -> "1.<minor>"), by parsing go.mod the same way, instead of a
// hardcoded constant - confirmed live that drifts silently: this sat at "1.31.0" while go.mod had
// already moved to k8s.io/api v0.36.0, so `go test` and `make test-integration` were testing
// against two different real Kubernetes API server versions without any error or warning.
func envtestK8sVersion() (string, error) {
	data, err := os.ReadFile(filepath.Join("..", "..", "go.mod"))
	if err != nil {
		return "", err
	}
	m := regexp.MustCompile(`k8s\.io/api\s+v\d+\.(\d+)\.\d+`).FindSubmatch(data)
	if m == nil {
		return "", fmt.Errorf("could not find a k8s.io/api version line in go.mod")
	}
	return "1." + string(m[1]), nil
}

// resolveEnvTestBinDir gets envtest's etcd/kube-apiserver/kubectl for the current OS by invoking
// setup-envtest, the same tool the Makefile's "test"/"setup-envtest" targets already use - so
// `make test` and a plain `go test` end up resolving binaries the same way. This runs
// `go run ...setup-envtest@latest`, which compiles the tool natively for whatever OS/arch it's
// invoked on - no committed per-OS binaries, no permission quirks (unlike cross-fetching one OS's
// binaries from another's tooling, which does not reliably carry over the executable bit).
// Confirmed live this works identically on Windows: setup-envtest logs a harmless
// "unable to remove downloaded archive" error to stderr on Windows (a file-locking artifact of
// its own temp-file cleanup, unrelated to whether the fetch itself succeeded) but still exits 0
// with the correct binaries extracted and the resolved path on stdout - cmd.Output() below only
// ever sees stdout, so that stderr noise never reaches the caller. An earlier version special-cased
// Windows to a vendored "bin-win/setup-envtest.exe" binary, believing `go run` didn't work there;
// that was never actually true, and it made this test suite dependent on the vendored binary
// existing (and staying in the repo's history) rather than fetching the same tool every OS uses.
//
// If KUBEBUILDER_ASSETS is already set (e.g. by `make test` itself), this is skipped entirely -
// controller-runtime's own BinPathFinder gives that env var precedence over BinaryAssetsDirectory
// regardless of what's returned here, so resolving it twice would just waste a subprocess call.
func resolveEnvTestBinDir() string {
	if os.Getenv("KUBEBUILDER_ASSETS") != "" {
		return ""
	}

	// setup-envtest appends its own "k8s/<version>-<os>-<arch>" under whatever --bin-dir is given
	// (confirmed live) - passing ".../k8s" here would double up into ".../k8s/k8s/...". Also must
	// be absolute: setup-envtest echoes "--bin-dir" back verbatim in its "-p path" output rather
	// than resolving it itself, and a relative path here would silently make
	// killOrphanedEnvtestProcesses's later match against Windows' (always-absolute)
	// Win32_Process ExecutablePath never fire - leaking an orphaned etcd/kube-apiserver on every
	// run (confirmed live).
	absBinDir, err := filepath.Abs(filepath.Join("..", "..", "bin"))
	if err != nil {
		logf.Log.Error(err, "failed to resolve envtest bin dir")
		return ""
	}

	k8sVersion, err := envtestK8sVersion()
	if err != nil {
		logf.Log.Error(err, "failed to resolve envtest k8s version from go.mod")
		return ""
	}

	cmd := exec.Command("go", "run", "sigs.k8s.io/controller-runtime/tools/setup-envtest@latest",
		"use", k8sVersion, "--bin-dir", absBinDir, "-p", "path")
	out, err := cmd.Output()
	if err != nil {
		logf.Log.Error(err, "failed to resolve envtest binaries via setup-envtest")
		return ""
	}
	return strings.TrimSpace(string(out))
}
