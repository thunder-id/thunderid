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
 * SVG trust-triangle diagram showing the Issuer → Holder → Verifier flow
 * used in Verifiable Credentials / Decentralized Identity documentation.
 *
 *          [Issuer]
 *         ↙ ① Issues VC        ③ Resolves DID Doc ↗ (dashed)
 * [Holder]  ──── ② Presents VP ────────────────→  [Verifier]
 */
export function TrustTriangleDiagram() {
  return (
    <div className="tt-diagram">
      <svg
        className="tt-diagram__svg"
        viewBox="0 0 800 510"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label="Verifiable Credentials trust triangle: Issuer, Holder wallet, and Verifier relying party"
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

        {/* ── Issuer — top centre, 240×80 ──────────────────────── */}
        <g className="tt-diagram__issuer" transform="translate(280,30)">
          <rect width="240" height="80" rx="10" />
          <text x="120" y="46" textAnchor="middle" className="tt-diagram__box-title">
            Issuer
          </text>
        </g>

        {/* ── Holder — bottom left, 220×80 ─────────────────────── */}
        <g className="tt-diagram__holder" transform="translate(40,390)">
          <rect width="220" height="80" rx="10" />
          <text x="110" y="34" textAnchor="middle" className="tt-diagram__box-title">
            Holder
          </text>
          <text x="110" y="54" textAnchor="middle" className="tt-diagram__box-sub">
            Wallet / App
          </text>
        </g>

        {/* ── Verifier — bottom right, 220×80 ──────────────────── */}
        <g className="tt-diagram__verifier" transform="translate(540,390)">
          <rect width="220" height="80" rx="10" />
          <text x="110" y="34" textAnchor="middle" className="tt-diagram__box-title">
            Verifier
          </text>
          <text x="110" y="54" textAnchor="middle" className="tt-diagram__box-sub">
            Relying Party
          </text>
        </g>

        {/* ── Edges ────────────────────────────────────────────── */}
        <g className="tt-diagram__edges">
          {/* ① Issuer → Holder: Issues VC (left side of triangle) */}
          <line x1="336" y1="110" x2="218" y2="390" markerEnd="url(#tt-arrow)" />
          <text x="168" y="248" textAnchor="middle" className="tt-diagram__edge-label">
            <tspan className="tt-diagram__edge-num">①</tspan> Issues VC
          </text>

          {/* ② Holder → Verifier: Presents VP (bottom of triangle) */}
          <line x1="260" y1="430" x2="540" y2="430" markerEnd="url(#tt-arrow)" />
          <text x="400" y="488" textAnchor="middle" className="tt-diagram__edge-label">
            <tspan className="tt-diagram__edge-num">②</tspan> Presents Verifiable Presentation
          </text>

          {/* ③ Verifier → Issuer: Resolves DID Doc (right side, dashed — offline) */}
          <line
            x1="575"
            y1="390"
            x2="464"
            y2="110"
            strokeDasharray="6,4"
            markerEnd="url(#tt-arrow)"
          />
          <text x="640" y="240" textAnchor="middle" className="tt-diagram__edge-label">
            <tspan className="tt-diagram__edge-num">③</tspan> Resolves DID Doc
          </text>
          <text x="640" y="256" textAnchor="middle" className="tt-diagram__edge-label">
            &amp; checks revocation
          </text>
        </g>
      </svg>
    </div>
  );
}
