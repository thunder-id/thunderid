/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import './WayfinderDiagrams.css';

// Reusable person-silhouette icon. PersonIcon viewBox is 56x56; the
// returned <g> renders within whatever transform / scale the caller
// applies. The outer circle is themed via the `className` prop; the
// inner glyph uses a fixed class so the same person silhouette renders
// in both diagrams.
function PersonIcon({className = undefined}: {className?: string}) {
  return (
    <g className={className}>
      <circle cx="28" cy="28" r="26" />
      <g transform="translate(28,28)" className="uc-b2c-wayfinder-person-glyph uc-agent-wayfinder-person-glyph">
        <circle cx="0" cy="-6" r="7" />
        <path d="M -13 14 C -13 4 13 4 13 14 Z" />
      </g>
    </g>
  );
}

/**
 * "Meet Wayfinder" diagram. The top card names the app; the two
 * columns below show consumers (peers, stacked) and staff (Alex at
 * the top with hierarchy connectors down to Sam and Maya).
 * Hierarchy lines align with the staff icons' horizontal centres.
 */
export function WayfinderOrganization() {
  return (
    <div className="uc-b2c-wayfinder-org">
      <svg
        className="uc-b2c-wayfinder-org__svg"
        viewBox="0 0 960 660"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder organization structure"
      >
        {/* Wayfinder header */}
        <g className="uc-b2c-wayfinder-org__header">
          <rect x="280" y="20" width="400" height="80" rx="12" />
          <text x="480" y="52" textAnchor="middle" className="uc-b2c-wayfinder-org__header-title">
            Wayfinder
          </text>
          <text x="480" y="78" textAnchor="middle" className="uc-b2c-wayfinder-org__header-sub">
            Consumer Travel-Booking Application
          </text>
        </g>

        {/* Trunk connectors splitting into two columns */}
        <g className="uc-b2c-wayfinder-org__edges">
          <line x1="480" y1="100" x2="480" y2="122" />
          <line x1="260" y1="122" x2="700" y2="122" />
          <line x1="260" y1="122" x2="260" y2="148" />
          <line x1="700" y1="122" x2="700" y2="148" />
        </g>

        {/* Consumers column — peers, stacked vertically */}
        <g className="uc-b2c-wayfinder-org__col uc-b2c-wayfinder-org__col--consumers" transform="translate(80,148)">
          <rect width="360" height="492" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-b2c-wayfinder-org__col-title">
            Consumers
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-b2c-wayfinder-org__col-sub">
            Book travel
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-b2c-wayfinder-org__divider" />

          {/* John Doe (top) */}
          <g transform="translate(156,95)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon" />
            </g>
          </g>
          <text x="180" y="171" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            John Doe
          </text>
          <text x="180" y="191" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            Returning traveller
          </text>

          {/* Jane Smith (middle) */}
          <g transform="translate(156,215)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon" />
            </g>
          </g>
          <text x="180" y="291" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            Jane Smith
          </text>
          <text x="180" y="311" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            Returning traveller
          </text>

          {/* Emma Wilson (bottom) */}
          <g transform="translate(156,335)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon" />
            </g>
          </g>
          <text x="180" y="411" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            Emma Wilson
          </text>
          <text x="180" y="431" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            New arrival via email
          </text>
        </g>

        {/* Staff column — hierarchy */}
        <g className="uc-b2c-wayfinder-org__col uc-b2c-wayfinder-org__col--staff" transform="translate(520,148)">
          <rect width="360" height="492" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-b2c-wayfinder-org__col-title">
            Staff
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-b2c-wayfinder-org__col-sub">
            Run the product
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-b2c-wayfinder-org__divider" />

          {/* Alex Carter (centered at the top of the column) */}
          <g transform="translate(156,110)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon uc-b2c-wayfinder-org__icon--lead" />
            </g>
          </g>
          <text x="180" y="186" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            Alex Carter
          </text>
          <text x="180" y="206" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            Operations admin
          </text>

          {/* Hierarchy connector — all lines aligned with icon centres */}
          <g className="uc-b2c-wayfinder-org__edges">
            <line x1="180" y1="220" x2="180" y2="252" />
            <line x1="110" y1="252" x2="250" y2="252" />
            <line x1="110" y1="252" x2="110" y2="280" />
            <line x1="250" y1="252" x2="250" y2="280" />
          </g>

          {/* Sam Rivera (left report, icon centre at x=110) */}
          <g transform="translate(86,280)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon" />
            </g>
          </g>
          <text x="110" y="356" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            Sam Rivera
          </text>
          <text x="110" y="376" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            Support agent
          </text>

          {/* Maya Patel (right report, icon centre at x=250) */}
          <g transform="translate(226,280)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-b2c-wayfinder-org__icon" />
            </g>
          </g>
          <text x="250" y="356" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-name">
            Maya Patel
          </text>
          <text x="250" y="376" textAnchor="middle" className="uc-b2c-wayfinder-org__cast-role">
            Destinations curator
          </text>
        </g>
      </svg>
    </div>
  );
}

/**
 * Cast diagram for the B2C Try It Out section. Shows the consumer and staff
 * personas used across the walkthroughs, with Alex Carter as the lead admin
 * above Sam Rivera and Maya Patel.
 */
export function WayfinderCast() {
  return (
    <div className="uc-b2c-wayfinder-cast">
      <svg
        className="uc-b2c-wayfinder-cast__svg"
        viewBox="0 0 960 492"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder cast: consumers and staff"
      >
        {/* Consumers section */}
        <rect x="20" y="12" width="3" height="16" className="uc-b2c-wayfinder-cast__group-accent" />
        <text x="30" y="24" className="uc-b2c-wayfinder-cast__group-label">Consumers</text>

        <g className="uc-b2c-wayfinder-cast__card" transform="translate(20, 36)">
          <rect width="280" height="148" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">John Doe</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Returning traveller</text>
          <line x1="16" y1="69" x2="264" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Signs in, manages his profile,</text>
          <text x="16" y="114" className="uc-b2c-wayfinder-cast__card-desc">and recovers his password</text>
        </g>

        <g className="uc-b2c-wayfinder-cast__card" transform="translate(340, 36)">
          <rect width="280" height="148" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">Jane Smith</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Returning traveller</text>
          <line x1="16" y1="69" x2="264" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Signs in and books trips</text>
          <text x="16" y="114" className="uc-b2c-wayfinder-cast__card-desc">as an existing customer</text>
        </g>

        <g className="uc-b2c-wayfinder-cast__card" transform="translate(660, 36)">
          <rect width="280" height="148" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">Emma Wilson</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">New traveller</text>
          <line x1="16" y1="69" x2="264" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Signs up with email and password</text>
        </g>

        {/* Staff section */}
        <rect x="20" y="212" width="3" height="16" className="uc-b2c-wayfinder-cast__group-accent" />
        <text x="30" y="224" className="uc-b2c-wayfinder-cast__group-label">Staff</text>

        <g className="uc-b2c-wayfinder-cast__card uc-b2c-wayfinder-cast__card--lead" transform="translate(20, 236)">
          <rect width="920" height="106" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">Alex Carter</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Operations admin</text>
          <line x1="16" y1="69" x2="904" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Onboards internal staff from the console</text>
        </g>

        <g className="uc-b2c-wayfinder-cast__card" transform="translate(20, 358)">
          <rect width="440" height="118" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">Sam Rivera</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Support agent</text>
          <line x1="16" y1="69" x2="424" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Helps users with support requests</text>
        </g>

        <g className="uc-b2c-wayfinder-cast__card" transform="translate(500, 358)">
          <rect width="440" height="118" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">Maya Patel</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Destinations curator</text>
          <line x1="16" y1="69" x2="424" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Manages destination content</text>
        </g>
      </svg>
    </div>
  );
}

/**
 * Cast diagram for the Decentralized Identity (VC) Try It Out. The tryout
 * centres on one member — John Doe — who receives the Sky Pass and presents it
 * at the lounge. Reuses the b2c cast card styling.
 */
export function WayfinderVcCast() {
  return (
    <div className="uc-b2c-wayfinder-cast">
      <svg
        className="uc-b2c-wayfinder-cast__svg"
        viewBox="0 0 960 176"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder cast: the Sky Pass holder"
      >
        <rect x="20" y="12" width="3" height="16" className="uc-b2c-wayfinder-cast__group-accent" />
        <text x="30" y="24" className="uc-b2c-wayfinder-cast__group-label">Member</text>

        <g className="uc-b2c-wayfinder-cast__card uc-b2c-wayfinder-cast__card--lead" transform="translate(20, 36)">
          <rect width="920" height="128" rx="10" />
          <text x="16" y="32" className="uc-b2c-wayfinder-cast__card-name">John Doe</text>
          <text x="16" y="56" className="uc-b2c-wayfinder-cast__card-role">Wayfinder member · Gold tier</text>
          <line x1="16" y1="69" x2="904" y2="69" className="uc-b2c-wayfinder-cast__card-divider" />
          <text x="16" y="92" className="uc-b2c-wayfinder-cast__card-desc">Signs in to Wayfinder and adds the Sky Pass to his wallet,</text>
          <text x="16" y="114" className="uc-b2c-wayfinder-cast__card-desc">then presents it at the Skyline Lounge for access</text>
        </g>
      </svg>
    </div>
  );
}

/**
 * Architecture diagram. Consumers (John, Jane, Emma) sit at the top next
 * to the Wayfinder Web app; ThunderID and Wayfinder Server sit below
 * the app, symmetrically. Pattern-agnostic — the arrow labels do not
 * commit to redirect-based vs app-native vs direct API.
 */
export function WayfinderArchitecture() {
  return (
    <div className="uc-b2c-wayfinder-arch">
      <svg
        className="uc-b2c-wayfinder-arch__svg"
        viewBox="0 0 960 560"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder app, server, and ThunderID integration"
      >
        <defs>
          <marker
            id="uc-b2c-arch-arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
          </marker>
        </defs>

        {/* Consumers — compact icons above Wayfinder Web */}
        <g className="uc-b2c-wayfinder-arch__consumers">
          <text x="240" y="24" textAnchor="middle" className="uc-b2c-wayfinder-arch__group-label">Consumers</text>
          <g transform="translate(122,34)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="140" y="92" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">John</text>
          <g transform="translate(222,34)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="240" y="92" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">Jane</text>
          <g transform="translate(322,34)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="340" y="92" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">Emma</text>
        </g>

        {/* Consumers → Wayfinder Web */}
        <g className="uc-b2c-wayfinder-arch__edges">
          <line x1="240" y1="94" x2="240" y2="162" markerEnd="url(#uc-b2c-arch-arrow)" />
        </g>

        {/* Wayfinder Web — left column */}
        <g className="uc-b2c-wayfinder-arch__app" transform="translate(60,162)">
          <rect width="340" height="115" rx="12" />
          <text x="170" y="28" textAnchor="middle" className="uc-b2c-wayfinder-arch__app-title">Wayfinder Web</text>
          <text x="170" y="50" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">Where consumers sign in and book</text>
          <line x1="16" y1="62" x2="324" y2="62" className="uc-b2c-wayfinder-arch__divider" />
          <text x="170" y="80" textAnchor="middle" className="uc-b2c-wayfinder-arch__box-desc">Delegates auth to ThunderID;</text>
          <text x="170" y="97" textAnchor="middle" className="uc-b2c-wayfinder-arch__box-desc">carries tokens to the API</text>
        </g>

        {/* WW ↔ ThunderID: OIDC / OAuth 2.0 */}
        <g className="uc-b2c-wayfinder-arch__edges">
          <line x1="400" y1="219" x2="540" y2="219" markerEnd="url(#uc-b2c-arch-arrow)" markerStart="url(#uc-b2c-arch-arrow)" />
          <text x="470" y="201" textAnchor="middle" className="uc-b2c-wayfinder-arch__edge-label">Sign-in flow</text>
          <text x="470" y="214" textAnchor="middle" className="uc-b2c-wayfinder-arch__edge-label">OIDC / OAuth 2.0</text>
        </g>

        {/* ThunderID — right column */}
        <g className="uc-b2c-wayfinder-arch__idp" transform="translate(540,162)">
          <rect width="320" height="115" rx="12" />
<text x="160" y="28" textAnchor="middle" className="uc-b2c-wayfinder-arch__idp-title">ThunderID</text>
          <text x="160" y="50" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">Identity provider</text>
          <line x1="16" y1="62" x2="304" y2="62" className="uc-b2c-wayfinder-arch__divider" />
          <text x="160" y="92" textAnchor="middle" className="uc-b2c-wayfinder-arch__box-desc">Signs users in and issues tokens</text>
        </g>

        {/* WW → Server: access token */}
        <g className="uc-b2c-wayfinder-arch__edges">
          <line x1="200" y1="277" x2="200" y2="406" markerEnd="url(#uc-b2c-arch-arrow)" />
          <text x="214" y="334" className="uc-b2c-wayfinder-arch__edge-label">Authenticated</text>
          <text x="214" y="349" className="uc-b2c-wayfinder-arch__edge-label">API calls</text>
        </g>

        {/* Wayfinder Server — left column, row 2 */}
        <g className="uc-b2c-wayfinder-arch__app" transform="translate(60,406)">
          <rect width="340" height="115" rx="12" />
          <text x="170" y="28" textAnchor="middle" className="uc-b2c-wayfinder-arch__app-title">Wayfinder Server</text>
          <text x="170" y="50" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">Booking API</text>
          <line x1="16" y1="62" x2="324" y2="62" className="uc-b2c-wayfinder-arch__divider" />
          <text x="170" y="80" textAnchor="middle" className="uc-b2c-wayfinder-arch__box-desc">Validates access tokens and</text>
          <text x="170" y="97" textAnchor="middle" className="uc-b2c-wayfinder-arch__box-desc">serves booking data</text>
        </g>

        {/* Staff → ThunderID: console access */}
        <g className="uc-b2c-wayfinder-arch__edges">
          <line x1="700" y1="318" x2="700" y2="279" markerEnd="url(#uc-b2c-arch-arrow)" />
          <text x="714" y="303" className="uc-b2c-wayfinder-arch__edge-label">Admin console</text>
        </g>

        {/* Staff / Admins — below ThunderID */}
        <g className="uc-b2c-wayfinder-arch__consumers">
          <text x="700" y="334" textAnchor="middle" className="uc-b2c-wayfinder-arch__group-label">Staff / Admins</text>
          <g transform="translate(578,346)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="596" y="403" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">Alex</text>
          <g transform="translate(678,346)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="696" y="403" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">Sam</text>
          <g transform="translate(778,346)"><g transform="scale(0.65)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
          <text x="796" y="403" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">Maya</text>
        </g>
      </svg>
    </div>
  );
}

/**
 * One flow panel for the VC architecture. A dashed boundary box labelled with
 * the trust-triangle role (Issuer / Verifier) encloses both the app and
 * ThunderID, showing that ThunderID powers the role from the inside. The
 * delegation arrow sits mid-box in the gap between the two inner nodes so it
 * never touches a title or divider. External arrows connect ThunderID directly
 * to the wallet (Holder) below the boundary.
 *
 * Coordinate notes (all in panel-local space, i.e. after translate(20,y)):
 *   App box:        translate(66, 46)  w=292  h=92  → right edge x=358, bottom y=138
 *   ThunderID box:  translate(552, 46) w=278  h=92  → left edge x=552,  bottom y=138
 *   Gap:            x=358–552  (194 px)
 *   Titles:         panel y=80  (base 46 + relative 34)
 *   Subs:           panel y=118 (base 46 + relative 72)
 *   Delegate label: panel y=90  — 10 px below title, 28 px above sub
 *   Delegate arrow: panel y=100 — 10 px below label, 18 px above sub
 *   Boundary:       y=34–156   (height 122)
 *   Wallet:         translate(415, 168)
 */
function VcFlowPanel({
  y,
  sectionLabel,
  roleLabel,
  appTitle,
  appSub,
  idpSub,
  delegateLabel,
  edgeAppWallet,
  edgeWalletThunder,
}: {
  y: number;
  sectionLabel: string;
  roleLabel: string;
  appTitle: string;
  appSub: string;
  idpSub: string;
  delegateLabel: string;
  edgeAppWallet: string;
  edgeWalletThunder: string;
}) {
  return (
    <g transform={`translate(20,${y})`}>
      {/* section header */}
      <rect x="4" y="0" width="3" height="16" className="uc-b2c-wayfinder-cast__group-accent" />
      <text x="14" y="13" className="uc-b2c-wayfinder-cast__group-label">{sectionLabel}</text>

      {/* Role label sits just above the boundary's top-right corner */}
      <text x="860" y="30" textAnchor="end" className="uc-vc-arch__boundary-label">{roleLabel}</text>

      {/* Dashed boundary — encloses App + ThunderID as the Issuer / Verifier */}
      <g className="uc-vc-arch__boundary">
        <rect x="50" y="34" width="820" height="122" rx="14" />
      </g>

      {/* App node (left, inside boundary) */}
      <g className="uc-b2c-wayfinder-arch__app" transform="translate(66,46)">
        <rect width="292" height="92" rx="12" />
        <text x="146" y="40" textAnchor="middle" className="uc-b2c-wayfinder-arch__app-title">{appTitle}</text>
        <text x="146" y="60" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">{appSub}</text>
      </g>

      {/* ThunderID node (right, inside boundary) */}
      <g className="uc-b2c-wayfinder-arch__idp" transform="translate(552,46)">
        <rect width="278" height="92" rx="12" />
        <text x="139" y="40" textAnchor="middle" className="uc-b2c-wayfinder-arch__idp-title">ThunderID</text>
        <text x="139" y="60" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">{idpSub}</text>
      </g>

      {/* Delegation arrow in the gap (x 358–552) */}
      <g className="uc-vc-arch__delegate">
        <text x="455" y="90" textAnchor="middle" className="uc-vc-arch__delegate-label">{delegateLabel}</text>
        <line x1="362" y1="100" x2="548" y2="100" markerEnd="url(#uc-vc-arch-arrow)" />
      </g>

      {/* Wallet / Holder (below boundary which ends at y=156) */}
      <g className="uc-b2c-wayfinder-arch__consumers">
        <g transform="translate(415,168)"><g transform="scale(0.72)"><PersonIcon className="uc-b2c-wayfinder-arch__icon" /></g></g>
        <text x="435" y="234" textAnchor="middle" className="uc-b2c-wayfinder-arch__cast-name">John&apos;s wallet</text>
        <text x="435" y="252" textAnchor="middle" className="uc-b2c-wayfinder-arch__sub">Holder</text>
      </g>

      {/* External edges */}
      <g className="uc-b2c-wayfinder-arch__edges">
        {/* App → Wallet (QR display) */}
        <line x1="198" y1="138" x2="414" y2="190" markerEnd="url(#uc-vc-arch-arrow)" />
        <text x="272" y="176" textAnchor="middle" className="uc-b2c-wayfinder-arch__edge-label">{edgeAppWallet}</text>
        {/* ThunderID ↔ Wallet (protocol) */}
        <line x1="456" y1="190" x2="656" y2="138" markerEnd="url(#uc-vc-arch-arrow)" markerStart="url(#uc-vc-arch-arrow)" />
        <text x="610" y="176" textAnchor="middle" className="uc-b2c-wayfinder-arch__edge-label">{edgeWalletThunder}</text>
      </g>
    </g>
  );
}

/**
 * Architecture diagrams for the Decentralized Identity (VC) Try It Out — one
 * trust triangle per flow (issuance, then verification). Each shows the three
 * components and how they connect, with plain arrows and no step-by-step
 * labels. Reuses the b2c architecture styling.
 */
export function WayfinderVcArchitecture() {
  return (
    <div className="uc-b2c-wayfinder-arch">
      <svg
        className="uc-b2c-wayfinder-arch__svg"
        viewBox="0 0 960 540"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Sky Pass issuance and verification architecture — ThunderID powers the Issuer and Verifier roles, with John's wallet as the Holder"
      >
        <defs>
          <marker
            id="uc-vc-arch-arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
          </marker>
        </defs>

        <VcFlowPanel
          y={10}
          sectionLabel="Issuance · OpenID4VCI"
          roleLabel="Issuer"
          appTitle="Wayfinder Web"
          appSub="Presents offer, shows QR"
          idpSub="Authenticates, signs, issues"
          delegateLabel="delegates OID4VCI"
          edgeAppWallet="Credential QR"
          edgeWalletThunder="Claim credential"
        />
        <VcFlowPanel
          y={272}
          sectionLabel="Verification · OpenID4VP"
          roleLabel="Verifier"
          appTitle="Skyline Lounge"
          appSub="Requests proof, shows QR"
          idpSub="Validates presentation"
          delegateLabel="delegates OID4VP"
          edgeAppWallet="Request QR"
          edgeWalletThunder="Present credential"
        />
      </svg>
    </div>
  );
}
// Agent-silhouette icon for the AI chat agent. Same outer-circle
// convention as PersonIcon so the two glyphs sit side-by-side at the
// same size; the inner glyph is a small bot face to distinguish the
// agent visually from human cast members.
function AgentIcon({className = undefined}: {className?: string}) {
  return (
    <g className={className}>
      <circle cx="28" cy="28" r="26" />
      <g transform="translate(28,28)" className="uc-agent-wayfinder-agent-glyph">
        <line x1="0" y1="-18" x2="0" y2="-12" />
        <circle cx="0" cy="-19" r="2" />
        <rect x="-12" y="-12" width="24" height="20" rx="4" />
        <circle cx="-5" cy="-4" r="1.8" />
        <circle cx="5" cy="-4" r="1.8" />
        <line x1="-5" y1="3" x2="5" y2="3" />
      </g>
    </g>
  );
}

/**
 * "Meet Wayfinder" diagram for the AI agent use cases. The top card
 * names the app; the two columns below show consumers (John Doe and
 * Jane Smith as peers) and the AI agent (Wayfinder Concierge as a
 * first-class principal with its own credentials).
 */
export function WayfinderAgentOrganization() {
  return (
    <div className="uc-agent-wayfinder-org">
      <svg
        className="uc-agent-wayfinder-org__svg"
        viewBox="0 0 960 540"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder principals: consumers and the chat agent"
      >
        {/* Wayfinder header */}
        <g className="uc-agent-wayfinder-org__header">
          <rect x="240" y="20" width="480" height="80" rx="12" />
          <text x="480" y="52" textAnchor="middle" className="uc-agent-wayfinder-org__header-title">
            Wayfinder
          </text>
          <text x="480" y="78" textAnchor="middle" className="uc-agent-wayfinder-org__header-sub">
            Travel-Booking with an AI Chat Assistant
          </text>
        </g>

        {/* Trunk connectors splitting into two columns */}
        <g className="uc-agent-wayfinder-org__edges">
          <line x1="480" y1="100" x2="480" y2="122" />
          <line x1="260" y1="122" x2="700" y2="122" />
          <line x1="260" y1="122" x2="260" y2="148" />
          <line x1="700" y1="122" x2="700" y2="148" />
        </g>

        {/* Consumers column — peers, stacked vertically */}
        <g className="uc-agent-wayfinder-org__col uc-agent-wayfinder-org__col--consumers" transform="translate(80,148)">
          <rect width="360" height="372" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-agent-wayfinder-org__col-title">
            Consumers
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-agent-wayfinder-org__col-sub">
            Book travel and chat with the agent
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-agent-wayfinder-org__divider" />

          {/* John Doe (top) */}
          <g transform="translate(156,100)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-agent-wayfinder-org__icon" />
            </g>
          </g>
          <text x="180" y="176" textAnchor="middle" className="uc-agent-wayfinder-org__cast-name">
            John Doe
          </text>
          <text x="180" y="196" textAnchor="middle" className="uc-agent-wayfinder-org__cast-role">
            Full access (UI + chat)
          </text>

          {/* Jane Smith (bottom) */}
          <g transform="translate(156,230)">
            <g transform="scale(0.86)">
              <PersonIcon className="uc-agent-wayfinder-org__icon" />
            </g>
          </g>
          <text x="180" y="306" textAnchor="middle" className="uc-agent-wayfinder-org__cast-name">
            Jane Smith
          </text>
          <text x="180" y="326" textAnchor="middle" className="uc-agent-wayfinder-org__cast-role">
            UI only, no chat access
          </text>
        </g>

        {/* AI Agent column — a single principal */}
        <g className="uc-agent-wayfinder-org__col uc-agent-wayfinder-org__col--agent" transform="translate(520,148)">
          <rect width="360" height="372" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-agent-wayfinder-org__col-title">
            AI Agent
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-agent-wayfinder-org__col-sub">
            Acts for itself, or on behalf of a user
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-agent-wayfinder-org__divider" />

          {/* Wayfinder Concierge (centered) */}
          <g transform="translate(156,150)">
            <g transform="scale(0.86)">
              <AgentIcon className="uc-agent-wayfinder-org__icon uc-agent-wayfinder-org__icon--agent" />
            </g>
          </g>
          <text x="180" y="226" textAnchor="middle" className="uc-agent-wayfinder-org__cast-name">
            Wayfinder Concierge
          </text>
          <text x="180" y="246" textAnchor="middle" className="uc-agent-wayfinder-org__cast-role">
            Accesses MCP tools
          </text>
        </g>
      </svg>
    </div>
  );
}

/**
 * Architecture diagram for the AI agent sample. Consumers sit at the
 * top next to Wayfinder Web (which hosts the chat widget). Below the
 * web app, two services fulfil chat and booking requests: the AI
 * Agent API (the Wayfinder Concierge) and the Wayfinder Server (the
 * booking API, also exposed to the agent as MCP tools). ThunderID is
 * the identity authority that issues the three token types used in
 * the sample — user, M2M, and OBO.
 */
export function WayfinderAgentArchitecture() {
  return (
    <div className="uc-agent-wayfinder-arch">
      <svg
        className="uc-agent-wayfinder-arch__svg"
        viewBox="0 0 960 620"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Wayfinder web app, chat agent, booking server, and ThunderID"
      >
        <defs>
          <marker
            id="uc-agent-arch-arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
          </marker>
        </defs>

        {/* Consumers — top, above Wayfinder Web */}
        <g className="uc-agent-wayfinder-arch__consumers">
          <text x="290" y="32" textAnchor="middle" className="uc-agent-wayfinder-arch__group-label">
            Consumers
          </text>

          {/* John Doe */}
          <g transform="translate(218,46)">
            <g transform="scale(0.78)">
              <PersonIcon className="uc-agent-wayfinder-arch__icon" />
            </g>
          </g>
          <text x="240" y="116" textAnchor="middle" className="uc-agent-wayfinder-arch__cast-name">
            John Doe
          </text>

          {/* Jane Smith */}
          <g transform="translate(318,46)">
            <g transform="scale(0.78)">
              <PersonIcon className="uc-agent-wayfinder-arch__icon" />
            </g>
          </g>
          <text x="340" y="116" textAnchor="middle" className="uc-agent-wayfinder-arch__cast-name">
            Jane Smith
          </text>
        </g>

        {/* Arrow from consumers down to Wayfinder Web */}
        <g className="uc-agent-wayfinder-arch__edges">
          <line x1="290" y1="130" x2="290" y2="170" markerEnd="url(#uc-agent-arch-arrow)" />
        </g>

        {/* Wayfinder Web — middle-left */}
        <g className="uc-agent-wayfinder-arch__app" transform="translate(80,170)">
          <rect width="420" height="130" rx="12" />
          <text x="210" y="40" textAnchor="middle" className="uc-agent-wayfinder-arch__app-title">
            Wayfinder Web
          </text>
          <text x="210" y="64" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Browser SPA with chat widget
          </text>
          <line x1="40" y1="80" x2="380" y2="80" className="uc-agent-wayfinder-arch__divider" />
          <text x="210" y="104" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Book travel, chat with the agent
          </text>
        </g>

        {/* ThunderID — right column, full app-stack height */}
        <g className="uc-agent-wayfinder-arch__idp" transform="translate(700,170)">
          <rect width="220" height="370" rx="12" />
          <text x="110" y="46" textAnchor="middle" className="uc-agent-wayfinder-arch__idp-title">
            ThunderID
          </text>
          <text x="110" y="72" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Identity Authority
          </text>
          <line x1="30" y1="92" x2="190" y2="92" className="uc-agent-wayfinder-arch__divider" />
          <text x="110" y="124" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Manages identities
          </text>
          <text x="110" y="148" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            and issues tokens
          </text>
        </g>

        {/* AI Agent — bottom-left */}
        <g className="uc-agent-wayfinder-arch__svc uc-agent-wayfinder-arch__svc--agent" transform="translate(80,400)">
          <rect width="260" height="140" rx="12" />
          <text x="130" y="40" textAnchor="middle" className="uc-agent-wayfinder-arch__app-title">
            AI Agent
          </text>
          <text x="130" y="64" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Wayfinder Concierge
          </text>
          <line x1="30" y1="80" x2="230" y2="80" className="uc-agent-wayfinder-arch__divider" />
          <text x="130" y="110" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Drives the conversation
          </text>
        </g>

        {/* Wayfinder Server — bottom-center (REST + MCP surfaces) */}
        <g className="uc-agent-wayfinder-arch__svc" transform="translate(380,400)">
          <rect width="260" height="140" rx="12" />
          <text x="130" y="40" textAnchor="middle" className="uc-agent-wayfinder-arch__app-title">
            Wayfinder Server
          </text>
          <text x="130" y="64" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Booking API + MCP tools
          </text>
          <line x1="30" y1="80" x2="230" y2="80" className="uc-agent-wayfinder-arch__divider" />
          <text x="130" y="110" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Holds flights, hotels, bookings
          </text>
        </g>

        {/* Edges */}
        <g className="uc-agent-wayfinder-arch__edges">
          {/* Wayfinder Web ↔ ThunderID (sign-in, user token) */}
          <line x1="500" y1="220" x2="700" y2="220" markerEnd="url(#uc-agent-arch-arrow)" />
          <line x1="700" y1="250" x2="500" y2="250" markerEnd="url(#uc-agent-arch-arrow)" />

          {/* Wayfinder Web → AI Agent (chat) */}
          <line x1="170" y1="300" x2="170" y2="400" markerEnd="url(#uc-agent-arch-arrow)" />

          {/* Wayfinder Web ↔ Wayfinder Server (authenticated API calls) */}
          <line x1="430" y1="300" x2="495" y2="400" markerEnd="url(#uc-agent-arch-arrow)" />
          <line x1="525" y1="400" x2="460" y2="300" markerEnd="url(#uc-agent-arch-arrow)" />

          {/* AI Agent ↔ ThunderID (M2M / OBO tokens) — routed under the bottom row to avoid Wayfinder Server */}
          <polyline points="290,540 290,580 810,580 810,540" markerEnd="url(#uc-agent-arch-arrow)" />
          <polyline points="770,540 770,595 250,595 250,540" markerEnd="url(#uc-agent-arch-arrow)" />

          {/* AI Agent → Wayfinder Server (MCP tool calls) */}
          <line x1="340" y1="500" x2="380" y2="500" markerEnd="url(#uc-agent-arch-arrow)" />

          {/* Wayfinder Server → ThunderID (JWKS for JWT validation) */}
          <line x1="640" y1="450" x2="700" y2="465" markerEnd="url(#uc-agent-arch-arrow)" />
        </g>
      </svg>
    </div>
  );
}

function McpClientIcon({className = ''}: {className?: string}) {
  return (
    <g className={className}>
      <circle cx="28" cy="28" r="26" />
      <g transform="translate(28,28)" className="uc-agent-wayfinder-agent-glyph">
        <rect x="-12" y="-10" width="24" height="20" rx="3" />
        <line x1="-12" y1="-3" x2="-18" y2="-3" />
        <line x1="-12" y1="3" x2="-18" y2="3" />
        <line x1="12" y1="-3" x2="18" y2="-3" />
        <line x1="12" y1="3" x2="18" y2="3" />
        <circle cx="0" cy="0" r="3" />
      </g>
    </g>
  );
}

export function WayfinderMcpOrganization() {
  return (
    <div className="uc-agent-wayfinder-org">
      <svg
        className="uc-agent-wayfinder-org__svg"
        viewBox="0 0 960 540"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="In-product and external MCP clients reaching the Wayfinder MCP server"
      >
        {/* Header */}
        <g className="uc-agent-wayfinder-org__header">
          <rect x="240" y="20" width="480" height="80" rx="12" />
          <text x="480" y="52" textAnchor="middle" className="uc-agent-wayfinder-org__header-title">
            Wayfinder
          </text>
          <text x="480" y="78" textAnchor="middle" className="uc-agent-wayfinder-org__header-sub">
            Travel-Booking with an embedded MCP server
          </text>
        </g>

        {/* Trunk connectors */}
        <g className="uc-agent-wayfinder-org__edges">
          <line x1="480" y1="100" x2="480" y2="122" />
          <line x1="260" y1="122" x2="700" y2="122" />
          <line x1="260" y1="122" x2="260" y2="148" />
          <line x1="700" y1="122" x2="700" y2="148" />
        </g>

        {/* AI Agent column — Wayfinder Concierge */}
        <g className="uc-agent-wayfinder-org__col uc-agent-wayfinder-org__col--agent" transform="translate(80,148)">
          <rect width="360" height="372" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-agent-wayfinder-org__col-title">
            AI Agent
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-agent-wayfinder-org__col-sub">
            Built into the Wayfinder app
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-agent-wayfinder-org__divider" />

          <g transform="translate(156,150)">
            <g transform="scale(0.86)">
              <AgentIcon className="uc-agent-wayfinder-org__icon uc-agent-wayfinder-org__icon--agent" />
            </g>
          </g>
          <text x="180" y="226" textAnchor="middle" className="uc-agent-wayfinder-org__cast-name">
            Wayfinder Concierge
          </text>
          <text x="180" y="246" textAnchor="middle" className="uc-agent-wayfinder-org__cast-role">
            Calls MCP tools through chat
          </text>
        </g>

        {/* External MCP Client column — MCP Inspector */}
        <g className="uc-agent-wayfinder-org__col uc-agent-wayfinder-org__col--agent" transform="translate(520,148)">
          <rect width="360" height="372" rx="10" />
          <text x="180" y="38" textAnchor="middle" className="uc-agent-wayfinder-org__col-title">
            External MCP Client
          </text>
          <text x="180" y="60" textAnchor="middle" className="uc-agent-wayfinder-org__col-sub">
            Connects from outside the app
          </text>
          <line x1="40" y1="78" x2="320" y2="78" className="uc-agent-wayfinder-org__divider" />

          <g transform="translate(156,150)">
            <g transform="scale(0.86)">
              <McpClientIcon className="uc-agent-wayfinder-org__icon uc-agent-wayfinder-org__icon--agent" />
            </g>
          </g>
          <text x="180" y="226" textAnchor="middle" className="uc-agent-wayfinder-org__cast-name">
            MCP Inspector
          </text>
          <text x="180" y="246" textAnchor="middle" className="uc-agent-wayfinder-org__cast-role">
            Calls MCP tools directly
          </text>
        </g>
      </svg>
    </div>
  );
}

/**
 * Architecture diagram for the MCP Authorization tryout. The left column
 * stacks User → External MCP Client → Wayfinder Server in a symmetric
 * column centered at x=290; ThunderID is the identity authority on the
 * right. Labels stay abstract — no endpoints, no implementation notes.
 */
export function WayfinderMcpArchitecture() {
  const {siteConfig} = useDocusaurusContext();
  const productName =
    (siteConfig.customFields?.product as DocusaurusProductConfig | undefined)?.project.name ?? siteConfig.title;
  return (
    <div className="uc-agent-wayfinder-arch">
      <svg
        className="uc-agent-wayfinder-arch__svg"
        viewBox="0 0 960 620"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label={`External MCP client, Wayfinder Server, and ${productName}`}
      >
        <defs>
          <marker
            id="uc-mcp-arch-arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto-start-reverse"
          >
            <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
          </marker>
        </defs>

        {/* User — top of the left column */}
        <g className="uc-agent-wayfinder-arch__consumers">
          <text x="290" y="32" textAnchor="middle" className="uc-agent-wayfinder-arch__group-label">
            User
          </text>

          <g transform="translate(268,46)">
            <g transform="scale(0.78)">
              <PersonIcon className="uc-agent-wayfinder-arch__icon" />
            </g>
          </g>
          <text x="290" y="116" textAnchor="middle" className="uc-agent-wayfinder-arch__cast-name">
            John Doe
          </text>
        </g>

        {/* User → External MCP Client */}
        <g className="uc-agent-wayfinder-arch__edges">
          <line x1="290" y1="130" x2="290" y2="170" markerEnd="url(#uc-mcp-arch-arrow)" />
          <text x="304" y="156" className="uc-agent-wayfinder-arch__edge-label">
            Use
          </text>
        </g>

        {/* External MCP Client — middle of the left column */}
        <g className="uc-agent-wayfinder-arch__app" transform="translate(80,170)">
          <rect width="420" height="130" rx="12" />
          <text x="210" y="40" textAnchor="middle" className="uc-agent-wayfinder-arch__app-title">
            External MCP Client
          </text>
          <text x="210" y="64" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            MCP Inspector
          </text>
          <line x1="40" y1="80" x2="380" y2="80" className="uc-agent-wayfinder-arch__divider" />
          <text x="210" y="104" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Discovers, signs in, calls MCP tools
          </text>
        </g>

        {/* ThunderID — right column, full height of the middle stack */}
        <g className="uc-agent-wayfinder-arch__idp" transform="translate(700,170)">
          <rect width="220" height="370" rx="12" />
          <text x="110" y="46" textAnchor="middle" className="uc-agent-wayfinder-arch__idp-title">
            {productName}
          </text>
          <text x="110" y="72" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Identity Authority
          </text>
          <line x1="30" y1="92" x2="190" y2="92" className="uc-agent-wayfinder-arch__divider" />
          <text x="110" y="124" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Manages identities
          </text>
          <text x="110" y="148" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            and issues tokens
          </text>
        </g>

        {/* Wayfinder Server — bottom of the left column, mirrors External MCP Client */}
        <g className="uc-agent-wayfinder-arch__svc" transform="translate(80,400)">
          <rect width="420" height="140" rx="12" />
          <text x="210" y="40" textAnchor="middle" className="uc-agent-wayfinder-arch__app-title">
            Wayfinder Server
          </text>
          <text x="210" y="64" textAnchor="middle" className="uc-agent-wayfinder-arch__sub">
            Booking API + MCP tools
          </text>
          <line x1="40" y1="80" x2="380" y2="80" className="uc-agent-wayfinder-arch__divider" />
          <text x="210" y="110" textAnchor="middle" className="uc-agent-wayfinder-arch__detail">
            Holds flights, hotels, bookings
          </text>
        </g>

        {/* Edges */}
        <g className="uc-agent-wayfinder-arch__edges">
          {/* External MCP Client ↔ ThunderID */}
          <line x1="500" y1="220" x2="700" y2="220" markerEnd="url(#uc-mcp-arch-arrow)" />
          <line x1="700" y1="250" x2="500" y2="250" markerEnd="url(#uc-mcp-arch-arrow)" />
          <text x="600" y="212" textAnchor="middle" className="uc-agent-wayfinder-arch__edge-label">
            Sign in
          </text>
          <text x="600" y="272" textAnchor="middle" className="uc-agent-wayfinder-arch__edge-label">
            Issue tokens
          </text>

          {/* External MCP Client → Wayfinder Server */}
          <line x1="290" y1="300" x2="290" y2="400" markerEnd="url(#uc-mcp-arch-arrow)" />
          <text x="304" y="354" className="uc-agent-wayfinder-arch__edge-label">
            Call MCP tools
          </text>

          {/* Wayfinder Server → ThunderID */}
          <line x1="500" y1="470" x2="700" y2="470" markerEnd="url(#uc-mcp-arch-arrow)" />
          <text x="600" y="462" textAnchor="middle" className="uc-agent-wayfinder-arch__edge-label">
            Validate tokens
          </text>
        </g>
      </svg>
    </div>
  );
}
