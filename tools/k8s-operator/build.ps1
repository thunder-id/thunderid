#!/usr/bin/env pwsh
# Copyright 2026 The ThunderID Authors
# SPDX-License-Identifier: Apache-2.0
#
# Requires PowerShell 7+ (the ternary operator below isn't available in Windows PowerShell 5.1).
#
# Mirrors the Makefile's test/test-unit/test-integration targets for Windows, where `make` isn't
# available - same pattern this project's own ThunderID product repo uses (Makefile -> ./build.sh
# on Linux/macOS, ./build.ps1 standalone on Windows).
#
# Unlike the Makefile, this does NOT run manifests/generate first: those depend on controller-gen,
# a Linux ELF binary the Makefile downloads on demand (`go_install_tool`, into a gitignored bin/ -
# never committed) that Make always resolves to regardless of host OS. CRD YAML and deepcopy code
# are generated manually on Windows instead. Everything else - fmt, vet, the actual test run -
# works natively here.

[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [string]$Command,

    [Parameter(Position = 1)]
    [string]$GoTestFlags = $env:GOTESTFLAGS
)

$ErrorActionPreference = "Stop"

function Test-Unit {
    go fmt ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $packages = go list ./... | Where-Object { $_ -notmatch '/e2e$' }
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    $goArgs = @('test', '-v') + ($GoTestFlags ? $GoTestFlags.Split(' ') : @()) + $packages + @('-coverprofile', 'cover.out')
    & go @goArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

function Test-Integration {
    go fmt ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    go vet -tags=integration ./...
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    # No KUBEBUILDER_ASSETS export needed here (unlike the Makefile) - suite_test.go's
    # resolveEnvTestBinDir resolves the right envtest binaries itself via `go run
    # setup-envtest@latest`, the same fetch every OS uses.
    $goArgs = @('test', '-tags=integration', '-v') + ($GoTestFlags ? $GoTestFlags.Split(' ') : @()) +
        @('./internal/controller/...', '-coverprofile', 'cover.out')
    & go @goArgs
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}

switch ($Command) {
    'test-unit' { Test-Unit }
    'test-integration' { Test-Integration }
    'test' {
        Test-Unit
        Test-Integration
    }
    default {
        Write-Host "Usage: $($MyInvocation.MyCommand.Name) {test-unit|test-integration|test}"
        exit 1
    }
}
