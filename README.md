<img src="https://thunderid.dev/assets/images/readme/repo-banner.png" alt="ThunderID" width="100%" />

###

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![GitHub last commit](https://img.shields.io/github/last-commit/thunder-id/thunderid.svg)](https://github.com/thunder-id/thunderid/commits/main)
[![GitHub issues](https://img.shields.io/github/issues/thunder-id/thunderid.svg)](https://github.com/thunder-id/thunderid/issues)
[![codecov.io](https://codecov.io/github/thunder-id/thunderid/coverage.svg?branch=main)](https://codecov.io/github/thunder-id/thunderid?branch=main)
[![GitHub Release](https://img.shields.io/github/v/release/thunder-id/thunderid?color=blue)](https://github.com/thunder-id/thunderid/releases/latest)


ThunderID is a lightweight, open-source Identity and Access Management (IAM) engine built to secure access for humans, AI agents, and machines.

Designed for the agentic era, ThunderID provides a developer-first IAM platform and supporting tools for securing applications, APIs, services, and agent-driven workflows. It works across traditional and decentralized identity ecosystems, with post-quantum-ready security built in from the start.

Core design goals of ThunderID include:
- **Agent-native identity:** Manage AI agents as first-class identities with delegated authority, consent-aware access, traceability, and support for issuing verifiable credentials to agents. ThunderID also aims to expose IAM capabilities through interfaces that agents can use safely and programmatically.
- **Decentralized identity:** Bridge the adoption gap for relying parties by making it practical for service providers to consume, verify, and trust decentralized identity in real-world applications, including DIDs, verifiable credentials, digital wallets, trust registries, and issuer-verifier-holder interaction models.
- **Cloud-native IAM:** Provide a lightweight, containerized identity product that can run across on-premises and cloud environments, with declarative identity flows, policies, and configuration suitable for automation, versioning, and GitOps practices.
- **Post-quantum-safe security:** Build on a crypto-agile foundation where algorithms, key types, signing methods, and token protection mechanisms can evolve over time, including support for post-quantum-safe algorithms and hybrid transition approaches across key management, credential issuance, assertions, and secure service-to-service communication.


## Getting Started

Get started by exploring how ThunderID can be used to secure:
* Applications - by following [Securing B2C Application Guide](https://thunderid.dev/docs/next/use-cases/b2c/try-it-out)
* AI Agents - by following [Securing AI Agents Guide](https://thunderid.dev/docs/next/use-cases/ai-agents/try-it-out)
* MCP - by following [Securing MCP Guide](https://thunderid.dev/docs/next/use-cases/ai-agents/mcp-authorization/try-it-out)

 To learn more about overall requirements, solution patterns of these scenarios, refer to the [Use Cases](https://thunderid.dev/docs/next/use-cases/overview/) section.

Visit [Get ThunderID](https://thunderid.dev/docs/next/guides/getting-started/get-thunderid/) to learn more about installation methods.


## Architecture

<img src="https://thunderid.dev/assets/images/readme/architecture.png" alt="ThunderID Architecture" width="100%" />


## Features

* **Identity Management**
    * Humans, AI agents, and workloads as first-class identity types
    * Hierarchical organizational units (OUs) and groups

* **Standards**
    * OAuth 2.1 and OpenID Connect, with PAR and PKCE
    * Verifiable Credentials — OpenID4VCI (issuance) and OpenID4VP (verification)
    * WebAuthn / passkeys
    * IdP federation — Google, Microsoft, GitHub, and any OIDC or SAML provider

* **Decentralized Identity**
    * Issue Verifiable Credentials to user wallets from configurable credential templates
    * Verify presented credentials against presentation definitions and trust anchors
    * Use them on their own, or as part of an identity journey

* **User Journeys**
    * Login, registration, and recovery defined as journeys
    * 20+ built-in executors - password, passkey, OTP, social login, consent, and more
    * Orchestratable in the server or the application
    * Themeable end-user UI

* **Authorization**
    * Hierarchical resources with derived permissions
    * Role-based access control across users, agents, and applications
    * Consent management with user-facing review

* **Developer Experience**
    * Console UI, REST APIs, and SDKs
    * MCP server for managing and querying IAM from AI agents

* **Declarative and GitOps-Ready**
    * YAML resource definitions for every entity
    * Immutable runtime


## Star History

<div align="center">
  <a href="https://www.star-history.com/?repos=thunder-id%2Fthunderid&type=date&legend=top-left">
   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/chart?repos=thunder-id/thunderid&type=date&theme=dark&legend=top-left" width="500" />
     <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/chart?repos=thunder-id/thunderid&type=date&legend=top-left" width="500" />
     <img alt="Star History Chart" src="https://api.star-history.com/chart?repos=thunder-id/thunderid&type=date&legend=top-left" width="500" />
   </picture>
  </a>
</div>

## Contributing

Please refer to the [Contributing Guide](https://thunderid.dev/docs/next/community/overview) for the different ways to contribute to this project and the relevant guidelines.

For code contributions, refer to the [Contributing Code](https://thunderid.dev/docs/next/community/contributing/contributing-code/prerequisites) section for details on the prerequisites and instructions for running ThunderID in development mode.


## License

Licenses this source under the Apache License, Version 2.0 ([LICENSE](LICENSE)), You may not use this file except in compliance with the License.

---------------------------------------------------------------------------
(c) Copyright 2026 WSO2 LLC.
