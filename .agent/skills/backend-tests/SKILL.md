---
name: backend-tests
description: Test conventions for ThunderID. Use whenever writing, changing, or fixing Go unit tests under backend/, integration tests under tests/integration, or Console e2e tests under tests/e2e, generating mocks, or adding tests for a feature or bug fix.
allowed-tools: Read Write Edit Bash
---

# Tests

Read with [backend/AGENTS.md](../../../backend/AGENTS.md), which says which test targets to run.

## What to Test

- Cover every case you can with integration tests. They exercise the real server, so they carry the most weight.
- Unit tests are still required. They test one component with its dependencies mocked, the DB provider included. Do not build a database harness in a unit test: that is an integration scenario.
- A query with a Postgres variant needs coverage on Postgres too. Integration tests run against Postgres in CI.
- Cover the failure paths: store errors returning `InternalServerError`, each validation error, and read-only refusal for declared resources.
- For a bug fix, add a regression test that fails without the fix when the case can be reproduced in a test.
- Do not add test-only code to source files (test loaders, `ForTest` constructors, test flags). Mock the dependency or inject the behaviour from the test.

## Mocks

Never hand-write a mock, stub, or fake. Generate it with mockery:

| Config in `backend/` | For | Output |
|---|---|---|
| `.mockery.private.yml` | interfaces used only inside their package | `<Interface>_mock_test.go` in the package |
| `.mockery.public.yml` | interfaces other packages depend on | `backend/tests/mocks/<pkg>mock/` |

Register interfaces with one entry per package that lists them under `interfaces:`, not one entry per interface. Run `make mockery` and commit the output. Regenerate after any interface change. Never edit generated mocks.

## Suites

```go
type ExampleServiceTestSuite struct {
	suite.Suite
	mockStore *exampleStoreInterfaceMock
}

func TestExampleServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ExampleServiceTestSuite))
}
```

- A new test file uses a testify suite.
- Tests added to a file that has a suite are suite methods.
- The suite is named after the file it tests (`example_service_test.go` holds `ExampleServiceTestSuite`).
- Test methods generally don't need comments. Add one only when the test is not self-explanatory or handles a case differently.
- Use table-driven tests for similar cases, not lint suppressions.
- Tests that read the server runtime call `config.ResetServerRuntime()` in `SetupTest` and `TearDownTest`.

## Integration Tests

Testify suites in `tests/integration/<area>/`, run with `make test_integration`. Also add:

| Change | Add |
|---|---|
| Declarative or composite store mode | a fixture under `tests/integration/resources/declarative_resources/<type>/` and a declarative test in the area (Ex: `tests/integration/group/group_declarative_test.go`) |
| An exportable resource | coverage in `tests/integration/importexport` |
| Control Plane only code | tests in `tests/integration/controlplane`, run with `make test_integration_cp` |
| Management API behaviour that differs between the Data Plane and the Control Plane | a case in `tests/integration/mgmtauthz`, which runs under both `make test_integration` and `make test_integration_cp`; give each plane its status with `Same` or `Differ`, and act as a persona from `testutils` (`PersonaClient`, `RequirePlane`) |

## E2E Tests

- Console e2e tests are Playwright specs in `tests/e2e/tests/<feature>/`. Add or update them when a change affects a Console flow.
- Follow "Writing New Tests" in [tests/e2e/README.md](../../../tests/e2e/README.md): routes and messages in `configs/` and `constants/`, generated test data, a page object in `pages/<feature>/` registered in `fixtures/`, and `test`/`expect` imported from the fixtures.
- Prefer `data-testid` selectors. Keep assertions out of page objects.
- Run with `make test_e2e`, or a single spec with `npx playwright test <path>` from `tests/e2e`.
- CLI changes are covered by the CLI e2e suite in `tests/e2e-cli` (`make tools_test_cli_e2e`).
