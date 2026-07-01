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

import './TrustTriangleDiagram.css';

/**
 * VC Trust Triangle — matches the Wikipedia / W3C layout:
 *
 *              [ Holder ]           ← top centre
 *             ↗           ↘
 *    ① Issues VC       ② Presents VP
 *           ↗               ↘
 *   [Issuer]  ─ ─ Trust ─ ─  [Verifier]
 *              (no integration needed)
 */
export function TrustTriangleDiagram() {
  return (
    <div className="tt-diagram">
      <svg
        className="tt-diagram__svg"
        viewBox="0 0 800 450"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Verifiable Credentials trust triangle: Holder (top), Issuer (bottom-left), Verifier (bottom-right)"
      >
        <defs>
          <marker
            id="tt-arrow"
            viewBox="0 0 10 10"
            refX="9"
            refY="5"
            markerWidth="6"
            markerHeight="6"
            orient="auto"
          >
            <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
          </marker>
        </defs>

        {/* ── Holder — top centre, 180×115 ─────────────────────────── */}
        <g className="tt-diagram__node tt-diagram__holder" transform="translate(310,18)">
          <rect width="180" height="118" rx="14" />
          <circle cx="90" cy="44" r="30" className="tt-diagram__icon-bg tt-diagram__icon-bg--holder" />
          {/* Person icon */}
          <g transform="translate(78,32)" className="tt-diagram__icon tt-diagram__icon--holder">
            <circle cx="12" cy="8" r="4" />
            <path d="M4 20c0-4 8-4 8-4s8 0 8 4" />
          </g>
          <text x="90" y="94" textAnchor="middle" className="tt-diagram__box-title">Holder</text>
          <text x="90" y="110" textAnchor="middle" className="tt-diagram__box-sub">Wallet</text>
        </g>

        {/* ── Issuer — bottom left, 180×130 ───────────────────────── */}
        <g className="tt-diagram__node tt-diagram__issuer" transform="translate(40,285)">
          <rect width="180" height="130" rx="14" />
          <circle cx="90" cy="48" r="30" className="tt-diagram__icon-bg tt-diagram__icon-bg--issuer" />
          {/* Landmark / institution icon — triangular pediment + 4 columns + base */}
          <g transform="translate(78,36)" className="tt-diagram__icon tt-diagram__icon--issuer">
            <polygon points="12,2 3,11 21,11" />
            <line x1="6"  y1="11" x2="6"  y2="18" />
            <line x1="10" y1="11" x2="10" y2="18" />
            <line x1="14" y1="11" x2="14" y2="18" />
            <line x1="18" y1="11" x2="18" y2="18" />
            <line x1="3"  y1="22" x2="21" y2="22" />
          </g>
          <text x="90" y="100" textAnchor="middle" className="tt-diagram__box-title">Issuer</text>
          <text x="90" y="118" textAnchor="middle" className="tt-diagram__box-sub">Institution / Authority</text>
        </g>

        {/* ── Verifier — bottom right, 180×130 ────────────────────── */}
        <g className="tt-diagram__node tt-diagram__verifier" transform="translate(580,285)">
          <rect width="180" height="130" rx="14" />
          <circle cx="90" cy="48" r="30" className="tt-diagram__icon-bg tt-diagram__icon-bg--verifier" />
          {/* Organisation / verified-building icon — building with check badge */}
          <g transform="translate(78,36)" className="tt-diagram__icon tt-diagram__icon--verifier">
            <rect x="3" y="9" width="18" height="13" rx="1" />
            <path d="M8 9V6a4 4 0 0 1 8 0v3" />
            <line x1="12" y1="14" x2="12" y2="17" />
            <circle cx="12" cy="14" r="1" />
          </g>
          <text x="90" y="100" textAnchor="middle" className="tt-diagram__box-title">Verifier</text>
          <text x="90" y="118" textAnchor="middle" className="tt-diagram__box-sub">Relying Party</text>
        </g>

        {/* ── Trust line — Issuer ←→ Verifier (no direct integration) */}
        <line x1="220" y1="350" x2="580" y2="350" className="tt-diagram__trust-line" />
        <text x="400" y="336" textAnchor="middle" className="tt-diagram__trust-label">Trust</text>
        <text x="400" y="368" textAnchor="middle" className="tt-diagram__trust-sub">No integration needed</text>

        {/* ── Edges ────────────────────────────────────────────────── */}
        <g className="tt-diagram__edges">
          {/* ① Issuer → Holder */}
          <line x1="155" y1="285" x2="360" y2="136" markerEnd="url(#tt-arrow)" />
          <text x="195" y="205" textAnchor="middle" className="tt-diagram__edge-label">
            <tspan className="tt-diagram__edge-num">①</tspan> Issues VC
          </text>

          {/* ② Holder → Verifier */}
          <line x1="445" y1="136" x2="645" y2="285" markerEnd="url(#tt-arrow)" />
          <text x="605" y="205" textAnchor="middle" className="tt-diagram__edge-label">
            <tspan className="tt-diagram__edge-num">②</tspan> Presents VP
          </text>
        </g>
      </svg>
    </div>
  );
}
