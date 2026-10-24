---
name: backend
description: Go backend conventions for ThunderID. Use whenever implementing, fixing, or changing Go code under backend/, including adding a package, service, handler, store, error code, log line, constant, or helper; adding or extending a management resource (CRUD API, store modes, declarative resources, export and import); or adding a server configuration key.
allowed-tools: Read Write Edit Bash
---

# Backend Conventions

Read with [backend/AGENTS.md](../../../backend/AGENTS.md). Schema and query rules are in the `db` skill, test rules in the `backend-tests` skill.

Read every reference file whose row matches the change:

| The change... | Read |
|---|---|
| Adds a file or package, or moves code between files | `layout.md` |
| Adds an exported name, wires a dependency through `init.go`, or starts a background worker | `visibility.md` |
| Adds or changes an error | `errors.md` |
| Adds or changes a log line | `logging.md` |
| Adds any new code: scope, constants, types, helpers, names, comments, user-facing text | `idiom.md` |
| Adds a management resource, or a store mode, export, or import to one | `resource.md` |
| Adds or changes a server configuration key | `config.md` |

Before adding anything, open the closest existing package and match its shape. When a rule here and an older package disagree, follow the rule.
