// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import { redirect } from 'next/navigation';
import SignInAction from '@/components/SignInAction';
import { getSessionUser } from '@/lib/auth';

const SECTIONS = [
  { href: '#platform', label: 'Platform' },
  { href: '#workflow', label: 'Workflow' },
  { href: '#outcomes', label: 'Outcomes' },
];

const Brand = () => (
  <a className="brand" href="#top">
    <img className="brand-logo" src="/wayfinder-mark.svg" alt="" width={34} height={34} />
    Wayfinder
  </a>
);

export default async function LandingPage() {
  if (await getSessionUser()) {
    redirect('/workspace');
  }

  return (
    <div className="public-shell" id="top">
      <header className="public-topbar">
        <Brand />
        <nav className="public-nav-links" aria-label="Landing page sections">
          {SECTIONS.map((section) => (
            <a key={section.href} href={section.href}>
              {section.label}
            </a>
          ))}
        </nav>
        <div className="landing-actions">
          <SignInAction variant="secondary">Get started</SignInAction>
          <SignInAction variant="primary">Sign in</SignInAction>
        </div>
      </header>

      <main>
        <section className="hero">
          <div className="hero-copy">
            <p className="eyebrow">Enterprise Travel Management</p>
            <h1>Corporate travel control for every client workspace.</h1>
            <p className="hero-text">
              Coordinate flights, approvals, travel policies, and spend across multiple workspaces with a platform
              built for agencies and enterprise travel teams.
            </p>
            <div className="landing-actions">
              <SignInAction variant="primary">Get started</SignInAction>
              <SignInAction variant="secondary">Sign in</SignInAction>
            </div>
            <div className="hero-proof" aria-label="Platform highlights">
              <span>Multi-workspace controls</span>
              <span>Policy-aware booking</span>
              <span>Centralized spend review</span>
            </div>
          </div>

          <div className="travel-board" aria-label="Travel management preview">
            <div className="board-header">
              <span>Global Travel Desk</span>
              <strong>Live</strong>
            </div>
            <div className="metric-grid">
              <div>
                <span>Open requests</span>
                <strong>128</strong>
              </div>
              <div>
                <span>In policy</span>
                <strong>84%</strong>
              </div>
              <div>
                <span>Avg approval</span>
                <strong>2h</strong>
              </div>
            </div>
            <div className="route-map" aria-hidden="true">
              <span className="route-node" />
              <span className="route-line" />
              <span className="route-node route-node-end" />
            </div>
            <div className="route-card">
              <span>Next review</span>
              <strong>New York to Los Angeles</strong>
              <p>In-policy economy fare, 9 travelers, approval window closes at 4:30 PM.</p>
            </div>
            <div className="flight-list-preview">
              <div>
                <span>WSO2 APAC</span>
                <strong>12 travelers</strong>
                <em>Approved</em>
              </div>
              <div>
                <span>Northstar Labs</span>
                <strong>6 travelers</strong>
                <em>Review</em>
              </div>
            </div>
          </div>
        </section>

        <section className="public-section stats-band" aria-label="Travel operation metrics">
          <div>
            <strong>36</strong>
            <span>client workspaces managed from one control center</span>
          </div>
          <div>
            <strong>$1.8M</strong>
            <span>quarterly travel spend tracked against policy</span>
          </div>
          <div>
            <strong>9.4k</strong>
            <span>traveler profiles organized across client programs</span>
          </div>
        </section>

        <section id="platform" className="public-section">
          <div className="section-intro">
            <p className="eyebrow">Platform</p>
            <h2>Everything agencies need after the booking request arrives.</h2>
            <p>
              Wayfinder brings the operational pieces together so account managers, finance teams, and client admins
              can work from the same travel record.
            </p>
          </div>
          <div className="feature-grid">
            <article>
              <span>01</span>
              <h3>Company workspaces</h3>
              <p>Separate client teams, travelers, policies, and billing context without losing central visibility.</p>
            </article>
            <article>
              <span>02</span>
              <h3>Policy routing</h3>
              <p>
                Highlight fare exceptions, preferred cabins, advance-purchase gaps, and approval paths before tickets
                are issued.
              </p>
            </article>
            <article>
              <span>03</span>
              <h3>Spend operations</h3>
              <p>Track allocations, pending bookings, and upcoming renewals with finance-ready summaries.</p>
            </article>
          </div>
        </section>

        <section id="workflow" className="public-section workflow-section">
          <div className="workflow-panel">
            <p className="eyebrow">Workflow</p>
            <h2>From request intake to approved itinerary in one coordinated flow.</h2>
            <div className="timeline">
              <div>
                <strong>Capture</strong>
                <p>Travelers submit request details with dates, destination, project, and cost center.</p>
              </div>
              <div>
                <strong>Validate</strong>
                <p>Client policy, traveler details, and budget rules are checked before approval.</p>
              </div>
              <div>
                <strong>Confirm</strong>
                <p>Admins approve, finance reviews spend, and travelers receive the final itinerary.</p>
              </div>
            </div>
          </div>
          <aside className="operations-card" aria-label="Operations snapshot">
            <span>Today</span>
            <strong>24 trips ready for finance review</strong>
            <div>
              <p>
                <b>7</b> fare exceptions
              </p>
              <p>
                <b>11</b> preferred hotel matches
              </p>
              <p>
                <b>6</b> pending manager approvals
              </p>
            </div>
          </aside>
        </section>

        <section id="outcomes" className="public-section outcomes-section">
          <div>
            <p className="eyebrow">Outcomes</p>
            <h2>Keep travel programs moving without adding operational drag.</h2>
          </div>
          <div className="outcomes-grid">
            <article>
              <h3>Faster approvals</h3>
              <p>Give account teams the context they need to move compliant trips forward quickly.</p>
            </article>
            <article>
              <h3>Clearer client service</h3>
              <p>Track requests, travelers, policies, and spend by workspace without switching tools.</p>
            </article>
            <article>
              <h3>Cleaner spend reviews</h3>
              <p>Surface exceptions, allocation details, and upcoming travel commitments before finance closes.</p>
            </article>
          </div>
        </section>

        <section className="public-section">
          <div className="cta-section">
            <div>
              <p className="eyebrow">Wayfinder</p>
              <h2>Make corporate travel feel coordinated before the itinerary is even booked.</h2>
            </div>
            <div className="landing-actions">
              <SignInAction variant="primary">Create workspace</SignInAction>
              <SignInAction variant="secondary">Sign in</SignInAction>
            </div>
          </div>
        </section>
      </main>

      <footer className="public-footer">
        <Brand />
        <p>Enterprise travel operations for agencies, finance teams, and client administrators.</p>
        <nav aria-label="Footer">
          {SECTIONS.map((section) => (
            <a key={section.href} href={section.href}>
              {section.label}
            </a>
          ))}
        </nav>
      </footer>
    </div>
  );
}
