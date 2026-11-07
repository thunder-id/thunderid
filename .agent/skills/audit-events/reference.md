# Audit Events Reference

Templates and checklists for the `audit-events` skill. `Example` stands for your resource. Every template is copied from the user service in `main` or from PR #6492; check the real code before you copy.

## Resources to Claim

Claim one row per PR on #6320 before you start. The event names are planned, not final: confirm them in your claim comment, and add the names to the spec in the same PR. A name has two to four lowercase segments (`eventNamePattern` in `backend/internal/system/eventbus/constants.go`).

| Resource | Package | Planned events | Notes |
|---|---|---|---|
| Roles | `backend/internal/role` (`service.go`) | `role.create`, `role.update`, `role.delete` | `UpdateRoleWithPermissions` is the update. |
| Role assignments | `backend/internal/role` (`assignment_service.go`) | `role.assignment.add`, `role.assignment.remove` | New action verbs need `Action` constants. Claim with roles or separately. |
| Groups | `backend/internal/group` | `group.create`, `group.update`, `group.delete` | `CreateGroupByPath` is a second create path. |
| Group membership | `backend/internal/group` | `group.member.add`, `group.member.remove` | `AddGroupMembers`, `RemoveGroupMembers`, `AddMembersToGroups`. |
| Agents | `backend/internal/agent` | `agent.create`, `agent.update`, `agent.delete` | An agent carries client credentials: never record them. |
| User types | `backend/internal/entitytype` | `user_type.create`, `user_type.update`, `user_type.delete` | One service serves `/user-types` and `/agent-types`; pick the name by category. |
| Agent types | `backend/internal/entitytype` | `agent_type.create`, `agent_type.update`, `agent_type.delete` | Same service as user types; one PR can cover both. |
| Connections and identity providers | `backend/internal/connection`, `backend/internal/idp` | `connection.create`, `connection.update`, `connection.delete` | Identity provider routes live in the connection package. Never record client secrets or keys. |
| Resource servers and permissions | `backend/internal/resource` | `resource_server.create`, `resource_server.update`, `resource_server.delete`, and `resource_server.resource.*`, `resource_server.action.*` | Resources and actions are the permissions. |
| Organization units | `backend/internal/ou` | `ou.create`, `ou.update`, `ou.delete` | |
| Projects | `backend/internal/project` | `project.create`, `project.update`, `project.delete` | Wired in the Control Plane only. |
| Sharing changes | `backend/internal/sharing` and each owning service | `<resource>.sharing_policy.create`, `.update`, `.delete` | User and application sharing policies are recorded gaps today. |
| Flows | `backend/internal/flow/mgt` | `flow.create`, `flow.update`, `flow.delete`, `flow.restore` | |
| Interceptors | `backend/internal/flow/interceptor` | none | Set in deployment configuration, with no management API. Nothing to audit until one exists. |
| Server configuration | `backend/internal/serverconfig` | `server_config.update` | |
| Themes and layouts | `backend/internal/design/theme/mgt`, `backend/internal/design/layout/mgt` | `theme.create`, `theme.update`, `theme.delete`, `layout.create`, `layout.update`, `layout.delete` | |
| Notification senders | `backend/internal/notification` (`mgt_service.go`) | `notification_sender.create`, `.update`, `.delete` | Sender properties hold credentials. |
| Notification templates | `backend/internal/notificationtemplate` | `notification_template.create`, `.update`, `.delete` | |
| Authorization details types | `backend/internal/authzdetail/mgt` | `authorization_detail_type.create`, `.update`, `.delete` | |
| Variables and secrets | `backend/internal/variablestore` | `variable.create`, `.update`, `.delete`, `secret.create`, `.update`, `.delete` | Server binary only. Keyed by name; never record a value. |
| Import, export, gateways | `backend/internal/system/importer`, `backend/internal/system/export`, `backend/internal/gateway` | to agree on #6320 | Import as an operation is a recorded gap. Each imported resource is audited by its own service. |

## Code Templates

`events.go`, beside the existing entries:

```go
const (
	// EventExampleCreate names an attempt to create an example.
	EventExampleCreate = "example.create"
	// EventExampleUpdate names an attempt to change an example.
	EventExampleUpdate = "example.update"
	// EventExampleDelete names an attempt to delete an example.
	EventExampleDelete = "example.delete"
)

const (
	// TargetTypeUser is the target type of an event about a user.
	TargetTypeUser = "user"
	// TargetTypeExample is the target type of an event about an example.
	TargetTypeExample = "example"
)

var auditedEvents = map[string][]string{
	// existing entries
	EventExampleCreate: nil,
	EventExampleUpdate: nil,
	EventExampleDelete: nil,
}
```

`error_constants.go` and `init.go`:

```go
// errNilEventBus is returned when the service is initialized without the event bus it audits on.
errNilEventBus = errors.New("the example service needs an event bus")

// Initialize ... bus is required: the service publishes the audit event of every example it creates,
// updates or deletes on it.
func Initialize(mux *http.ServeMux, /* existing parameters */, bus eventbus.EventBusInterface) (
	ExampleServiceInterface, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	exampleService := newExampleService(/* existing arguments */, bus)
```

`service.go`:

```go
// CreateExample creates the example.
func (es *exampleService) CreateExample(ctx context.Context, example *Example) (*Example,
	*tidcommon.ServiceError) {
	created, svcErr := es.createExample(ctx, example)
	var exampleID string
	if svcErr == nil {
		exampleID = created.ID
	}
	es.publishExampleEvent(ctx, audit.EventExampleCreate, audit.ActionCreate, exampleID, svcErr)
	return created, svcErr
}

// createExample creates the example without publishing its audit event.
func (es *exampleService) createExample(ctx context.Context, example *Example) (*Example,
	*tidcommon.ServiceError) {
	// the former body of CreateExample
}

// UpdateExample updates the example.
func (es *exampleService) UpdateExample(ctx context.Context, id string, example *Example) (*Example,
	*tidcommon.ServiceError) {
	updated, svcErr := es.updateExample(ctx, id, example)
	es.publishExampleEvent(ctx, audit.EventExampleUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// publishExampleEvent publishes the audit event of an attempt on the example with id, after the change
// committed or was refused.
func (es *exampleService) publishExampleEvent(ctx context.Context, name string, action audit.Action,
	id string, svcErr *tidcommon.ServiceError) {
	target := audit.Ref{Type: audit.TargetTypeExample, ID: id}
	eventbus.TryPublish(ctx, es.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}
```

Both `servicemanager.go` files:

```go
exampleService, err := example.Initialize(mux, /* existing arguments */, auditBus)
fatalOnError(ctx, logger, err, "Failed to initialize ExampleService")
```

## Test Templates

Unit tests go in the package. Tests added to a file that has a suite are suite methods; #6492 puts them in `service_audit_test.go` as methods of the existing service suite. Copy `newRecordingBus`, `recordingSink` and `discardingBus` from `backend/internal/user/service_test.go`.

```go
func (suite *ServiceTestSuite) TestCreateExample_PublishesTheCreatedExample() {
	bus, published := newRecordingBus(suite.T())
	// creatingExampleService and adminContext are your own helpers, shaped like creatingUserService
	// and adminContext in backend/internal/user/service_test.go.
	service := creatingExampleService(suite.T(), bus)

	created, svcErr := service.CreateExample(adminContext(), &Example{ /* realistic values */ })

	suite.Require().Nil(svcErr)
	events := published()
	suite.Require().Len(events, 1)
	ref := audit.Ref{Type: audit.TargetTypeExample, ID: created.ID}
	suite.Equal(audit.EventExampleCreate, events[0].name)
	suite.Equal(ref, events[0].payload.Target())
	suite.Equal(ref, events[0].payload.Subject())
	suite.Equal(audit.OutcomeSucceeded, events[0].payload.Outcome())
	suite.Empty(events[0].payload.Details())
}
```

A failed case asserts `audit.OutcomeFailed("<code in lower case with underscores>")`, the target with the caller's ID (or no ID on create) and `audit.Ref{}` as the subject. A failing bus uses `eventbusmock.NewEventBusInterfaceMock` with `EXPECT().Publish(...).Return(errors.New(...))` and an exact call count.

Integration tests go on `AuditLogTestSuite` in `tests/integration/audit`, in a file of their own (#6492 adds `application_audit_test.go`):

```go
created, body := ts.exchange(testutils.GetHTTPClient(), http.MethodPost, "/examples", requestBody)
ts.Require().Equal(http.StatusCreated, created.StatusCode, string(body))
// update and delete the same way, keeping each response
lines := ts.linesOf(created, updated, deleted)
// assert Event, Action, Target, Subject, Actor, Outcome on each line
// and ts.NotContains(line.Raw, secretOrName) for every value sent
```

Prefix every handle and name with `ts.prefix` unless the column is short; check the column width in `backend/dbscripts/*/postgres.sql` first.

## Running on Postgres

CI runs the Postgres suite in `.github/workflows/postgres-tests.yml` on a schedule, not on every PR, so run it yourself:

1. Start Postgres on `localhost:5432` with user `dbuser` and password `dbpassword`.
2. Create the four databases and load their scripts, as the "Setup PostgreSQL Database" step of `.github/actions/run-integration-tests/action.yml` does (`backend/dbscripts/<database>/postgres.sql`).
3. Run `DB_TYPE=postgres ./tests/integration/resources/scripts/setup-test-config.sh` from the repository root. It rewrites the tracked `tests/integration/resources/deployment.yaml`; restore it with `git checkout tests/integration/resources/deployment.yaml` afterwards and never commit the change.
4. Run `make build_backend`, then `DB_TYPE=postgres make test_integration PACKAGE=./audit`.

## Known Limitations for the PR Description

```markdown
### Known limitations
- Declarative and file-backed <resources>, and direct database edits, are not recorded.
- Requests refused by the security layer with 401 or 403, and requests rejected before they reach the <resource> service, are not recorded.
- <Only if the delete is idempotent:> A delete of a missing ID is recorded as `succeeded`.
- The event bus is best effort and can drop events under overload.
- <Any path into the change that does not reach the service, named.>
```
