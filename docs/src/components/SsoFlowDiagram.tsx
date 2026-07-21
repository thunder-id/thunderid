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

import React from 'react';

export function SsoSingleStepDiagram(): React.ReactElement {
  return (
    <figure
      className="flow-node-diagram"
      role="img"
      aria-label="Single-step Single Sign-On flow: Start leads to Check SSO Session. The Skip to path goes straight to Save / Load Session; the Authenticate path goes through Collect Credentials and Credentials Auth before reaching Save / Load Session. Save / Load Session then leads to Auth Assertion Generator and End."
    >
      <svg
        viewBox="0 0 910 180"
        style={{width: '100%', overflow: 'visible', display: 'block', fontFamily: 'inherit'}}
        aria-hidden="true"
      >
        <defs>
          <marker id="sso1-arr" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto">
            <polygon points="0 0, 8 3, 0 6" style={{fill: 'context-stroke'}} />
          </marker>
        </defs>

        {/* ── Nodes ──────────────────────────────────────────────────── */}
        <rect x="12" y="33" width="60" height="34" rx="17" className="fnd-node fnd-node--start" />
        <text x="42" y="50" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">
          Start
        </text>

        <rect x="96" y="28" width="132" height="44" rx="8" className="fnd-node fnd-node--task" />
        <text x="162" y="50" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Check SSO Session
        </text>

        <rect x="470" y="28" width="140" height="44" rx="8" className="fnd-node fnd-node--task" />
        <text x="540" y="50" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Save / Load Session
        </text>

        <rect x="648" y="28" width="150" height="44" rx="8" className="fnd-node fnd-node--task" />
        <text x="723" y="44" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Auth Assertion
        </text>
        <text x="723" y="58" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Generator
        </text>

        <rect x="832" y="33" width="60" height="34" rx="17" className="fnd-node fnd-node--end" />
        <text x="862" y="50" textAnchor="middle" dominantBaseline="central" className="fnd-label fnd-label--light">
          End
        </text>

        <rect x="250" y="120" width="140" height="40" rx="6" className="fnd-node fnd-node--prompt" />
        <text x="320" y="140" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Collect Credentials
        </text>

        <rect x="410" y="120" width="120" height="40" rx="6" className="fnd-node fnd-node--task" />
        <text x="470" y="140" textAnchor="middle" dominantBaseline="central" className="fnd-label">
          Credentials Auth
        </text>

        {/* ── Arrows ─────────────────────────────────────────────────── */}
        <line x1="72" y1="50" x2="92" y2="50" className="fnd-edge" markerEnd="url(#sso1-arr)" />

        {/* Check → Save (Skip to) */}
        <line x1="228" y1="50" x2="466" y2="50" className="fnd-edge fnd-edge--success" markerEnd="url(#sso1-arr)" />

        {/* Check → Collect Credentials (Authenticate) */}
        <path d="M 162,72 V 140 H 246" className="fnd-edge fnd-edge--failure" markerEnd="url(#sso1-arr)" />

        {/* Collect Credentials → Credentials Auth */}
        <line x1="390" y1="140" x2="406" y2="140" className="fnd-edge" markerEnd="url(#sso1-arr)" />

        {/* Credentials Auth → Save (rejoin) */}
        <path d="M 530,140 H 540 V 74" className="fnd-edge" markerEnd="url(#sso1-arr)" />

        {/* Save → Auth Assertion Generator */}
        <line x1="610" y1="50" x2="644" y2="50" className="fnd-edge" markerEnd="url(#sso1-arr)" />

        {/* Auth Assertion Generator → End */}
        <line x1="798" y1="50" x2="828" y2="50" className="fnd-edge" markerEnd="url(#sso1-arr)" />

        {/* ── Edge labels ────────────────────────────────────────────── */}
        <text x="347" y="42" textAnchor="middle" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--success">
          Skip to
        </text>
        <text x="170" y="104" dominantBaseline="central" className="fnd-edge-label fnd-edge-label--failure">
          Authenticate
        </text>
      </svg>
    </figure>
  );
}
