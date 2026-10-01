# Adopters

If your organization is using ThunderID, or your open source project integrates with it, we kindly encourage you to add your name to this list. This helps demonstrate real-world usage, builds community confidence, and strengthens the project's momentum.

| Organization Name | Link | Date of First Use | Description |
|---|---|---|---|
| [LSF](https://opensource.lk) | [OpenNSW](https://github.com/OpenNSW) | 2026-02 | Open-source Digital Public Infrastructure building blocks for National Single Window systems, where applicants complete multi-agency approvals through one entry point. OpenNSW recommends ThunderID as the identity provider for Single Windows built with it. Its first implementation, [nsw-srilanka](https://github.com/OpenNSW/nsw-srilanka) (Sri Lanka's Trade Single Window), uses ThunderID to sign in traders, agency officers, and administrators with OIDC, and to secure calls between the Single Window and agency systems with OAuth2 client credentials. |
| [LSF](https://opensource.lk) | [OpenGovMail](https://github.com/OpenGovMail/OpenGovMail/blob/main/README.md) | 2025-08 | Open source, self-hosted email platform for governments and organizations. OpenGovMail ships ThunderID as its default identity provider, so every mailbox is a ThunderID user or a role. The OpenGovMail IMAP/LMTP server authenticates users against ThunderID with username/password and OAUTHBEARER, and administrators manage users, organization units, groups and roles in the ThunderID console. |
| [OpenChoreo](https://openchoreo.dev) | [Identity configuration](https://openchoreo.dev/docs/platform-engineer-guide/identity-configuration/) | 2025-10 | [CNCF Sandbox](https://www.cncf.io/projects/openchoreo/) developer platform for Kubernetes. OpenChoreo ships ThunderID as its default identity provider, providing OAuth2 and OIDC authentication for its Backstage-powered developer portal, the `occ` CLI, its AI agents, and its MCP servers, along with the platform's users, groups, and client applications. |
| [WSO2](https://wso2.com/) | [WSO2 Agent Manager](https://wso2.com/agent-platform/agent-manager/) | 2025-12 | Open control plane for deploying, managing, observing, and governing AI agents. WSO2 Agent Manager uses ThunderID as its default identity provider, providing OAuth 2.0 and OpenID Connect authentication for platform users and applications. ThunderID also enables AgentID support in Agent Manager. See the [AgentID documentation](https://wso2.com/agent-platform/docs/v1.0.0/concepts/agentid/). |

---
## Adding Your Organization

1. Open a pull request that adds a row to the [Adopters](#adopters) table, keeping the table in alphabetical order.
2. Fill in every column:
   - **Organization Name**: the name of your organization.
   - **Link**: your organization's website, or a public case study, blog post, or talk about your use of ThunderID.
   - **Date of First Use**: the year, or year and month, you started using ThunderID.
   - **Description**: a short description of how you use ThunderID and the impact it has.

Self-submission is preferred, because a pull request from someone in the adopting organization gives explicit consent to be listed. If a maintainer opens the pull request on your behalf, a member of your organization must approve it in a pull request comment before it is merged.

To update or remove your entry, open a pull request.
