import React from 'react';

type ModelWorkspace = {
  name: string;
  items: string[];
};

type JourneyStep = {
  title: string;
  scenario: string;
  thunder: string;
  flow: string[];
};

const workspaces: ModelWorkspace[] = [
  {
    name: 'Customer Workspace A',
    items: [
      'Users',
      'Roles',
      'Policies',
      'Federation',
      'Recovery',
    ],
  },
  {
    name: 'Customer Workspace B',
    items: [
      'Users',
      'Roles',
      'Policies',
      'Branding',
    ],
  },
  {
    name: 'Customer Workspace C',
    items: [
      'Admins',
      'Invited users',
      'Governance',
    ],
  },
];

const journeySteps: JourneyStep[] = [
  {
    title: 'Create Customer Workspace',
    scenario: 'Set up the organization boundary for a customer.',
    thunder: 'Creates the workspace, assigns the initial admin, and applies policy, billing, and branding defaults.',
    flow: ['New customer signs up', 'Create workspace', 'Assign initial admin', 'Apply policy and billing'],
  },
  {
    title: 'Invite Collaborators',
    scenario: 'Bring customer admins and users into the workspace.',
    thunder: 'Handles invitation lifecycle, role assignment, and workspace-scoped access.',
    flow: ['Workspace admin', 'Sends invites', 'Members join', 'Scoped roles applied'],
  },
  {
    title: 'Evolve Identity',
    scenario: 'Move from direct sign-in to social and enterprise federation.',
    thunder: 'Adds social login, OIDC or SAML federation, JIT provisioning, and account linking.',
    flow: ['Direct sign-up', 'Add Google login', 'Enable enterprise SSO', 'JIT or account linking'],
  },
  {
    title: 'Recover Access',
    scenario: 'Let users regain access based on workspace policy.',
    thunder: 'Supports email magic links, email OTP, SMS OTP, and policy-driven recovery routes.',
    flow: ['Access issue detected', 'Recovery method selected', 'Policy checks', 'Access restored'],
  },
  {
    title: 'Govern Operations',
    scenario: 'Delegate administration, audit activity, and manage subscription changes.',
    thunder: 'Supports delegated admin, access governance, audit visibility, and operational controls.',
    flow: ['Delegate admin', 'Track activity', 'Apply governance rules', 'Adjust subscription access'],
  },
  {
    title: 'Prepare for Agent Access',
    scenario: 'Extend the same organization boundary to AI agents and workloads.',
    thunder: 'Uses the same workspace and policy model for future agent identity and control.',
    flow: ['Organization boundary', 'Agent identity added', 'Policy-based access'],
  },
];

export function B2BSaaSIdentityModelGraph() {
  return (
    <section className="uc-b2b-model" aria-label="B2B SaaS identity model">
      <div className="uc-b2b-model__root">
        <div className="uc-b2b-model__kicker">Platform Layer</div>
        <div className="uc-b2b-model__title">SaaS Platform</div>
        <div className="uc-b2b-model__stem"></div> {/* Added a dedicated stem element */}
      </div>
      <div className="uc-b2b-model__branches">
        {workspaces.map((workspace) => (
          <article key={workspace.name} className="uc-b2b-model__workspace">
            <h3>{workspace.name}</h3>
            <ul aria-label={`${workspace.name} controls`}>
              {workspace.items.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </article>
        ))}
      </div>
    </section>
  );
}

export function B2BSaaSIdentityJourneyTimeline() {
  return (
    <section className="uc-b2b-journey" aria-label="B2B SaaS identity journey timeline">
      {journeySteps.map((step, index) => (
        <article key={step.title} className="uc-b2b-journey__step">
          <div className="uc-b2b-journey__index" aria-hidden />
          <div className="uc-b2b-journey__content">
            <h3>{index + 1}. {step.title}</h3>
            <p>
              <strong>Scenario:</strong> {step.scenario}
            </p>
            <p>
              <strong>What ThunderID Handles:</strong> {step.thunder}
            </p>
            <div className="uc-b2b-journey__flow" role="list" aria-label={`${step.title} flow`}>
              {step.flow.map((flowNode, flowIndex) => (
                <React.Fragment key={flowNode}>
                  <span role="listitem" className="uc-b2b-journey__flow-node">
                    {flowNode}
                  </span>
                  {flowIndex < step.flow.length - 1 && <span className="uc-b2b-journey__flow-arrow">→</span>}
                </React.Fragment>
              ))}
            </div>
          </div>
        </article>
      ))}
    </section>
  );
}

export function B2BSaaSIdentityJourneyRoadmap() {
  return (
    <div className="uc-b2b-roadmap" role="navigation" aria-label="B2B use case roadmap">
      <svg className="uc-b2b-roadmap__path" viewBox="0 0 1000 700" preserveAspectRatio="none" aria-hidden="true">
        <path d="M500 92 V112" />
        <path d="M500 112 L116 112" />
        <path d="M500 112 L372 112" />
        <path d="M500 112 L628 112" />
        <path d="M500 112 L884 112" />
        <path d="M116 112 V124" />
        <path d="M372 112 V124" />
        <path d="M628 112 V124" />
        <path d="M884 112 V124" />
      </svg>
      <a href="#organization-onboarding-sign-up" className="uc-b2b-roadmap__node uc-b2b-roadmap__node--primary">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M5 13c0-4.5 3.2-7.8 7-9 3.8 1.2 7 4.5 7 9 0 4.2-3 7.6-7 9-4-1.4-7-4.8-7-9Z"/><path d="M12 8v8M8 12h8"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Onboard Organization</span>
      </a>

      <div className="uc-b2b-roadmap__category">
        <span className="uc-b2b-roadmap__category-label">Identity &amp; Access</span>
      </div>
      <div className="uc-b2b-roadmap__category">
        <span className="uc-b2b-roadmap__category-label">Administration</span>
      </div>
      <div className="uc-b2b-roadmap__category">
        <span className="uc-b2b-roadmap__category-label">Configuration</span>
      </div>
      <div className="uc-b2b-roadmap__category">
        <span className="uc-b2b-roadmap__category-label">Operations</span>
      </div>

      <a href="#organizational-identity-management" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M4 20V8l8-4 8 4v12"/><path d="M9 20v-5h6v5"/><path d="M8 10h.01M12 10h.01M16 10h.01"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Manage Identities</span>
      </a>
      <a href="#identity-recovery" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M7 11V9a5 5 0 0 1 10 0v2"/><rect x="5" y="11" width="14" height="9" rx="2"/><path d="M12 15v2"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Recover Identities</span>
      </a>
      <a href="#organization-workspace-authorization-and-subscription-controls" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M4 12h16"/><path d="M12 4v16"/><path d="M7 7h10v10H7z"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Authorize Access</span>
      </a>
      <a href="#ai-agents" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><rect x="7" y="8" width="10" height="8" rx="2"/><path d="M9 12h.01M15 12h.01"/><path d="M12 8V5"/><path d="M9 16v3M15 16v3"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Manage AI Agents</span>
      </a>

      <a href="#delegated-administration" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><circle cx="9" cy="8" r="3"/><path d="M4 19c0-3 2.3-5 5-5"/><path d="M15 8h5"/><path d="M17.5 5.5v5"/><path d="M13 19l2 2 5-5"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Delegate Administrators</span>
      </a>
      <a href="#collaboration" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><circle cx="9" cy="9" r="3"/><circle cx="15" cy="9" r="3"/><path d="M4 19c0-3 2.4-5 5-5s5 2 5 5"/><path d="M10 19c.4-2.2 2.2-4 4.5-4 2.6 0 4.5 2 4.5 4"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Collaborate with Organizations</span>
      </a>

      <a href="#organizational-sign-in" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M14 3h6v18h-6"/><path d="M10 12h10"/><path d="m7 9 3 3-3 3"/><path d="M4 4h8v16H4"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Configure Sign-In</span>
      </a>
      <a href="#branding-customization" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M12 4a8 8 0 1 0 0 16c1.4 0 2.5-1.1 2.5-2.5 0-1.1.9-2 2-2h1A4.5 4.5 0 0 0 22 11 7 7 0 0 0 12 4Z"/><circle cx="7.5" cy="11" r="1"/><circle cx="10.5" cy="8" r="1"/><circle cx="14" cy="8" r="1"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Customize Branding</span>
      </a>
      <a href="#organization-discovery-mechanisms" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/><path d="M12 4v8l4 2"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Configure Organization Discovery</span>
      </a>

      <a href="#privacy-and-compliance" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M12 4 5 7v5c0 4.3 2.9 7 7 8 4.1-1 7-3.7 7-8V7l-7-3Z"/><path d="M9.5 12h5"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Ensure Compliance</span>
      </a>
      <a href="#platform-monitoring-and-governance" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="M4 19h16"/><path d="M7 16v-4"/><path d="M12 16V8"/><path d="M17 16v-6"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Platform Audit</span>
      </a>
      <a href="#troubleshooting" className="uc-b2b-roadmap__node">
        <span className="uc-b2b-roadmap__icon" aria-hidden>
          <svg viewBox="0 0 24 24"><path d="m14 6 4 4"/><path d="M10 20a7 7 0 1 1 5.7-11.1"/><path d="M3 21l5-5"/></svg>
        </span>
        <span className="uc-b2b-roadmap__label">Troubleshoot Issues</span>
      </a>
    </div>
  );
}
