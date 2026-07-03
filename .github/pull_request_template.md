### Purpose
<!-- Describe the problem, feature, improvement or the change introduced by the PR briefly. Add screenshots/GIFs if UI/UX changes are introduced. -->

<!-- If this PR contains breaking changes, uncomment and fill in the section below -->
<!--

---
### ⚠️ Breaking Changes

#### 🔧 Summary of Breaking Changes
_Describe what is changing_

#### 💥 Impact
_What will break? Who is affected?_

#### 🔄 Migration Guide
_How should users update their code/configuration to adapt to the breaking changes? Include examples if helpful_

---

-->

### Approach
<!-- Describe how you are implementing the solution, what are the key design decisions and why. Add diagrams if necessary. -->

### Related Issues
- N/A

### Related PRs
- N/A

### Checklist
- [ ] Followed the contribution guidelines.
- [ ] Manual test round performed and verified.
- [ ] Documentation provided. (Add links if there are any)
    - [ ] Ran Vale and fixed all errors and warnings
- [ ] Tests provided. (Add links if there are any)
    - [ ] Unit Tests
    - [ ] Integration Tests
- [ ] Breaking changes. (Fill if applicable)
    - [ ] Breaking changes section filled.
    - [ ] `breaking change` label added.

### Security checks
- [ ] Followed secure coding standards in [WSO2 Secure Coding Guidelines](https://security.docs.wso2.com/en/latest/security-guidelines/secure-engineering-guidelines/secure-coding-guidlines/introduction/)
- [ ] Confirmed that this PR doesn't commit any keys, passwords, tokens, usernames, or other secrets.

### API production-readiness (only if this PR touches `api/*.yaml`)
<!-- The API quality gate enforces the mechanical items below in CI; these boxes are the
     human attestation. If a box is intentionally N/A, say why on the line. -->
- [ ] New/changed collection endpoints support pagination (and filtering/sorting, or a tracked exemption)
- [ ] Every operation declares an `operationId` and the standard error responses
- [ ] Write operations are idempotent and tenant-scoped (or a tracked exemption exists)
- [ ] Backward compatible, or a breaking change called out above with a version bump
- [ ] Contract tests added/updated for every changed operation (`tests/integration/contract`)
- [ ] No rule exemptions added, OR each new entry in `api-quality-gate/governance/exemptions.yaml`
      has a justification, owner, expiry, and tracking issue
