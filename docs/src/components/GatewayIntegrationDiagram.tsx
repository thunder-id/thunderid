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
import './GatewayIntegrationDiagram.css';

interface STSIntegrationDiagramProps {
  gatewayName: string;
  /** Optional SVG content rendered inside the gateway box (logo paths, etc.). */
  gatewayLogo?: React.ReactNode;
  /** Optional brand background colour for the gateway box (e.g. '#041c36'). */
  gatewayBgColor?: string;
}

export function STSIntegrationDiagram({gatewayName, gatewayLogo = undefined, gatewayBgColor = undefined}: STSIntegrationDiagramProps) {
  const {siteConfig} = useDocusaurusContext();
  const productName =
    (siteConfig.customFields?.product as DocusaurusProductConfig | undefined)
      ?.project.name ?? siteConfig.title;

  return (
    <div className="sts-diagram">
      <svg
        className="sts-diagram__svg"
        viewBox="0 0 960 410"
        xmlns="http://www.w3.org/2000/svg"
        role="img"
        aria-label={`${gatewayName} and ${productName} STS integration flow`}
      >
        <defs>
          <marker
            id="sts-arrow"
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

        {/* Application — top left, 240×80 */}
        <g className="sts-diagram__app" transform="translate(40,80)">
          <rect width="240" height="80" rx="10" />
          <text x="120" y="48" textAnchor="middle" className="sts-diagram__box-title">
            Application
          </text>
        </g>

        {/* ThunderID — right, 240×250 */}
        <g className="sts-diagram__idp" transform="translate(680,80)">
          <rect width="240" height="250" rx="10" />
          {/* logo-mini icon scaled to 36×45, centered horizontally */}
          <g transform="translate(102,90) scale(0.174)">
            <path d="M55.4763 26.4391L58.8866 0H0V26.4391H55.4763Z" className="sts-diagram__idp-logo-dark" />
            <path d="M39.8438 147.407L49.5455 72.2839H4.9909e-05V256.743H60.5602L80.048 147.407H39.8438Z" className="sts-diagram__idp-logo-accent" />
            <path d="M192.42 59.361C182.782 40.2307 168.929 25.5705 150.903 15.3381C145.501 12.2662 139.761 9.6605 133.703 7.5208L115.401 103.702H159.757L76.2987 256.743H83.3735C109.449 256.743 131.69 251.574 150.14 241.236C168.569 230.897 182.634 216.131 192.356 196.959C202.058 177.765 206.909 154.8 206.909 128.043C206.909 101.286 202.079 78.5123 192.441 59.3821L192.42 59.361Z" className="sts-diagram__idp-logo-accent" />
          </g>
          <text x="120" y="158" textAnchor="middle" className="sts-diagram__box-title sts-diagram__idp-name">
            {productName}
          </text>
        </g>

        {/* Gateway — below Application, 240×64 */}
        <g className="sts-diagram__gateway" transform="translate(40,200)">
          <rect
            width="240"
            height="64"
            rx="10"
            style={gatewayBgColor ? {fill: gatewayBgColor, stroke: gatewayBgColor} : undefined}
          />
          {gatewayLogo ?? (
            <text x="120" y="36" textAnchor="middle" className="sts-diagram__box-title">
              {gatewayName}
            </text>
          )}
        </g>

        {/* Backend API — below KrakenD, 240×60 */}
        <g className="sts-diagram__backend" transform="translate(40,324)">
          <rect width="240" height="60" rx="10" />
          <text x="120" y="36" textAnchor="middle" className="sts-diagram__box-title">
            Backend API
          </text>
        </g>

        {/* Edges */}
        <g className="sts-diagram__edges">
          {/* ① Application → ThunderID: client credentials grant */}
          <line x1="280" y1="100" x2="679" y2="100" markerEnd="url(#sts-arrow)" />
          <text x="480" y="90" textAnchor="middle" className="sts-diagram__edge-label">
            <tspan className="sts-diagram__edge-num">①</tspan> Request Access token
          </text>

          {/* ② ThunderID → Application: access token — 40px below ① */}
          <line x1="680" y1="140" x2="281" y2="140" markerEnd="url(#sts-arrow)" />
          <text x="480" y="130" textAnchor="middle" className="sts-diagram__edge-label">
            <tspan className="sts-diagram__edge-num">②</tspan> Issue Access token
          </text>

          {/* ③ Application → KrakenD: API request with bearer token */}
          <line x1="160" y1="160" x2="160" y2="199" markerEnd="url(#sts-arrow)" />
          <text x="172" y="176" className="sts-diagram__edge-label">
            <tspan className="sts-diagram__edge-num">③</tspan> API request
          </text>
          <text x="172" y="192" className="sts-diagram__edge-label">
            + Bearer token
          </text>

          {/* ④ KrakenD → ThunderID: direct horizontal from right edge */}
          <line x1="280" y1="232" x2="679" y2="232" markerEnd="url(#sts-arrow)" />
          <text x="480" y="222" textAnchor="middle" className="sts-diagram__edge-label">
            <tspan className="sts-diagram__edge-num">④</tspan> Call JWKS endpoint to validate token signature
          </text>

          {/* ⑤ KrakenD → Backend: straight down */}
          <line x1="160" y1="264" x2="160" y2="323" markerEnd="url(#sts-arrow)" />
          <text x="172" y="296" className="sts-diagram__edge-label">
            <tspan className="sts-diagram__edge-num">⑤</tspan> Forward request
          </text>
        </g>
      </svg>
    </div>
  );
}

/**
 * KrakenD logo (icon + wordmark) pre-sized for the 240×64 gateway box.
 * Pass as the `gatewayLogo` prop to GatewayIntegrationDiagram.
 */
export function KrakenDLogo({color = undefined}: {color?: string}) {
  return (
    <g
      className={color ? undefined : 'sts-diagram__krakend-logo'}
      fill={color}
      fillRule="nonzero"
      transform="translate(50,18) scale(0.891)"
    >
      <path d="M15.024.928c4.024 0 7.796 1.531 10.619 4.31 2.84 2.797 4.405 6.555 4.405 10.58 0 8.332-6.74 15.11-15.024 15.11-6.178 0-11.497-3.77-13.803-9.143a3.142 3.142 0 0 1-.059-.13A15.178 15.178 0 0 1 0 15.818c0-4.025 1.565-7.783 4.405-10.58C7.23 2.46 11 .929 15.024.929zm1.051 4.44-.201.029-.017.002.01-.001.007-.001.203-.023-.21.024-.014.002-.02.003.244-.03c-4.6.48-6.644 3.864-6.628 7.003.002.357.032.708.088 1.05a7.767 7.767 0 0 1 4.854-1.682c4.308 0 7.77 3.474 7.88 7.91.053 2.12-.676 4.023-2.108 5.5-1.628 1.68-3.992 2.644-6.694 2.743-.262.018-.52.023-.772.023-2.542 0-4.568-.502-6.168-1.221a13.644 13.644 0 0 0 8.495 2.96c7.588 0 13.762-6.21 13.762-13.841a13.78 13.78 0 0 0-.51-3.73l-.132-.441-.161-.28c-3.09-5.176-8.104-6.501-11.908-5.999zm-1.051-3.17c-3.692 0-7.15 1.402-9.736 3.948-2.596 2.556-4.025 5.991-4.025 9.672 0 1.941.4 3.79 1.12 5.469.42.855 3.04 5.482 10.624 5.36v-.012c2.551 0 4.772-.84 6.252-2.368 1.19-1.226 1.795-2.81 1.75-4.582a7.273 7.273 0 0 0-.102-1.05 7.77 7.77 0 0 1-4.84 1.67c-4.323 0-7.858-3.554-7.88-7.923-.022-4.318 3.063-7.678 7.531-8.245l-.076.011c2.636-.408 5.84-.023 8.722 1.727l.329.207-.204-.195c-2.554-2.382-5.9-3.69-9.465-3.69zm-4.598 17.829a2.6 2.6 0 0 1 2.59 2.604 2.6 2.6 0 0 1-2.59 2.604 2.6 2.6 0 0 1-2.59-2.604 2.6 2.6 0 0 1 2.59-2.604zm0 1.27c-.732 0-1.327.598-1.327 1.334 0 .735.596 1.334 1.327 1.334.732 0 1.327-.599 1.327-1.334 0-.736-.595-1.335-1.327-1.335zm-4.945-6.668a3.009 3.009 0 0 1 2.997 3.014 3.009 3.009 0 0 1-2.997 3.014 3.008 3.008 0 0 1-2.996-3.014 3.009 3.009 0 0 1 2.996-3.014zm20.023.057a2.6 2.6 0 0 1 2.589 2.605 2.6 2.6 0 0 1-2.59 2.604 2.6 2.6 0 0 1-2.589-2.604 2.6 2.6 0 0 1 2.59-2.605zM5.48 15.9c-.956 0-1.734.782-1.734 1.744 0 .961.778 1.744 1.734 1.744.957 0 1.735-.783 1.735-1.744 0-.962-.778-1.744-1.735-1.744zm8.91-2.886a6.535 6.535 0 0 0-4.487 1.764c.968 2.49 3.37 4.258 6.163 4.258a6.534 6.534 0 0 0 4.469-1.746c-.977-2.525-3.348-4.276-6.144-4.276zm11.113 2.943c-.732 0-1.327.598-1.327 1.334 0 .736.595 1.335 1.327 1.335.731 0 1.327-.599 1.327-1.335s-.596-1.334-1.327-1.334zm-3.276-8.049a3.009 3.009 0 0 1 2.997 3.014 3.008 3.008 0 0 1-2.997 3.014 3.008 3.008 0 0 1-2.997-3.014 3.009 3.009 0 0 1 2.997-3.014zM5.445 8.67a2.285 2.285 0 0 1 2.276 2.29 2.285 2.285 0 0 1-2.276 2.287 2.285 2.285 0 0 1-2.276-2.288A2.285 2.285 0 0 1 5.445 8.67zm16.783.508c-.956 0-1.734.783-1.734 1.744 0 .962.778 1.745 1.734 1.745s1.734-.783 1.734-1.745a1.74 1.74 0 0 0-1.734-1.744zM5.445 9.94c-.559 0-1.013.457-1.013 1.02 0 .561.454 1.018 1.013 1.018.559 0 1.013-.457 1.013-1.019 0-.562-.454-1.019-1.013-1.019zm10.293-3.424a2.285 2.285 0 0 1 2.275 2.289 2.285 2.285 0 0 1-2.275 2.288 2.285 2.285 0 0 1-2.276-2.288 2.285 2.285 0 0 1 2.276-2.29zm0 1.27c-.56 0-1.014.456-1.014 1.018s.455 1.019 1.014 1.019c.558 0 1.013-.457 1.013-1.019 0-.562-.455-1.019-1.013-1.019zM40.064 8.663l4.742-2.735v9.76l5.554-6.21h5.68l-6.366 6.719 6.583 10.35h-5.429l-4.368-6.975-1.654 1.784v5.19h-4.742zM57.285 9.477h4.743v3.44c.967-2.357 2.527-3.885 5.335-3.758v5.063h-.25c-3.15 0-5.085 1.943-5.085 6.019v6.305h-4.743V9.477zM67.3 21.642v-.064c0-3.726 2.777-5.445 6.74-5.445 1.685 0 2.901.286 4.087.7v-.286c0-2.006-1.217-3.12-3.588-3.12-1.81 0-3.09.35-4.618.923l-1.185-3.694c1.84-.828 3.65-1.37 6.49-1.37 2.589 0 4.461.701 5.647 1.911 1.248 1.274 1.81 3.153 1.81 5.445v9.904h-4.587V24.7c-1.155 1.306-2.746 2.166-5.055 2.166-3.15 0-5.74-1.848-5.74-5.223zm10.89-1.115v-.86c-.811-.382-1.873-.636-3.027-.636-2.028 0-3.276.828-3.276 2.356v.064c0 1.306 1.06 2.07 2.59 2.07 2.215 0 3.712-1.242 3.712-2.994zM85.048 8.663l4.743-2.735v9.76l5.554-6.21h5.678l-6.365 6.719 6.584 10.35h-5.43l-4.368-6.975-1.653 1.784v5.19h-4.743z" />
      <path d="M100.68 18.107v-.064c0-4.872 3.4-8.884 8.267-8.884 5.585 0 8.144 4.426 8.144 9.267 0 .382-.031.828-.063 1.273H105.39c.468 2.198 1.966 3.344 4.088 3.344 1.59 0 2.746-.51 4.056-1.75l2.714 2.45c-1.56 1.975-3.806 3.185-6.832 3.185-5.024 0-8.737-3.598-8.737-8.82zm11.793-1.432c-.28-2.166-1.529-3.63-3.526-3.63-1.966 0-3.244 1.432-3.619 3.63h7.145zM118.895 9.477h4.742v2.421c1.092-1.433 2.496-2.739 4.899-2.739 3.588 0 5.679 2.42 5.679 6.337v11.05h-4.743v-9.521c0-2.293-1.06-3.471-2.87-3.471s-2.965 1.178-2.965 3.47v9.522h-4.742V9.477zM141.763 5.928h3.511c6.491 0 10.976 4.315 10.976 9.943v.057c0 5.629-4.485 10-10.976 10h-8.054V9.899l4.543-3.971zm3.511 16.029c3.718 0 6.226-2.429 6.226-5.972v-.057c0-3.543-2.508-6.029-6.226-6.029h-3.51v12.058h3.51z" />
    </g>
  );
}

export function KongLogo({color = undefined}: {color?: string}) {
  const textStyle = color ? {fill: color} : undefined;
  return (
    <>
      {/* Icon: 24×24 viewBox scaled to 36×36, centred vertically in the 64px box */}
      <g transform="translate(50,14) scale(1.5)">
        <defs>
          <linearGradient id="sts-kong-grad" x1="0.02" y1="22.48" x2="18.6" y2="3.76" gradientUnits="userSpaceOnUse">
            <stop stopColor="#1155CB" />
            <stop offset="1" stopColor="#1DB57C" />
          </linearGradient>
        </defs>
        <path d="M12.3167 3.42016L11.3447 5.20295H13.756L17.8942 10.1091L20.3543 8.09634V6.81818L19.4997 5.62186L20.1313 4.9672L15.1954 1.07623L12.3167 3.42016Z" fill="url(#sts-kong-grad)" />
        <path d="M8.91553 9.66287L10.9592 6.09924L13.3333 6.09729L23.9993 18.6683L23.1721 22.4912H18.5998L18.8834 21.4138L8.91553 9.66287Z" fill="url(#sts-kong-grad)" />
        <path d="M7.28076 19.4594L7.86941 18.7151H12.2422L14.5147 21.5325L14.1235 22.4912H8.4737L8.61256 21.5325L7.28076 19.4594Z" fill="url(#sts-kong-grad)" />
        <path d="M3.54946 13.3512H4.89689L8.36815 10.3857L12.9756 15.8334L11.6673 17.8306H7.4177L4.48035 21.6202L3.80762 22.4912H0V17.8403L3.54946 13.3512Z" fill="url(#sts-kong-grad)" />
      </g>
      <text x="94" y="38" className="sts-diagram__box-title" style={textStyle}>Kong Konnect</text>
    </>
  );
}

export function EnvoyLogo({color = undefined}: {color?: string}) {
  const textStyle = color ? {fill: color} : undefined;
  return (
    <>
      {/* Icon: viewBox "-4.21 49.54 439.92 332.67", scaled to 36px tall */}
      <g transform="translate(72,14) scale(0.1082) translate(4.21,-49.54)">
        <path fill="#b31aab" d="M109.8 210.6l.6 25.4 26.8 16.6-.6-25.4zm65.4 105.8l-.6-24.9-23.5-14.6c-.3-.2-.7-.5-1-.7l.6 25 24.5 15.2zM91.5 350l-61.3-38-1.5-63.7 30.1-13-.6-25.5-48 20.7c-3.7 1.6-5.9 5-5.8 8.9l1.8 76.5c.1 3.9 2.5 7.8 6.3 10.2L86 371.7c3.4 2.1 7.6 2.7 11 1.6.4-.1.7-.2 1-.4l45.1-19.4-24.5-15.2L91.5 350z" />
        <path fill="#d163ce" d="M289.6 209.1c-.1-4.6-2.9-9.1-7.3-11.9L193 141.9l-2.8 1.2.6 26.8 70.7 43.8 1.7 71.6 27 16.7 1.5-.6-2.1-92.3zM182.7 334.8l-82.9-51.4-2-86.3 37.8-16.3-.7-29.7-58.7 25.3c-4.3 1.9-6.9 5.8-6.8 10.4L71.7 288c.1 4.6 2.9 9.1 7.3 11.8l97.2 60.3c4 2.5 8.8 3.1 12.9 1.9.4-.1.8-.3 1.2-.5l57.4-24.7-28.6-17.7-36.4 15.7z" />
        <path fill="#e13eaf" d="M415.9 138.3L291.3 61c-4.6-2.8-10.1-3.6-14.8-2.1-.5.1-.9.3-1.4.5l-121.6 52.4c-4.9 2.1-7.9 6.6-7.8 11.9l3.1 129.6c.1 5.3 3.3 10.4 8.4 13.5L281.8 344c4.6 2.8 10.1 3.6 14.7 2.1.5-.1.9-.3 1.4-.5l121.6-52.4c4.9-2.1 7.9-6.7 7.8-11.9l-3-129.6c-.1-5.1-3.3-10.3-8.4-13.4zM289.3 315.2L181 248.1l-2.7-112.7 105.6-45.5L392.2 157l2.7 112.7-105.6 45.5z" />
      </g>
      <text x="128" y="38" className="sts-diagram__box-title" style={textStyle}>Envoy</text>
    </>
  );
}

export function APISIXLogo({color = undefined}: {color?: string}) {
  const textStyle = color ? {fill: color} : undefined;
  return (
    <>
      {/* Hexagon mark with APISIX "A", 36×42px positioned in the 240×64 gateway box */}
      <g transform="translate(48,11)">
        <polygon points="18,0 36,9 36,33 18,42 0,33 0,9" fill="#E0314B" />
        <text x="18" y="30" textAnchor="middle" fill="#fff" fontSize="18" fontWeight="bold" fontFamily="sans-serif">A</text>
      </g>
      <text x="98" y="38" className="sts-diagram__box-title" style={textStyle}>Apache APISIX</text>
    </>
  );
}

export function AzureAPIMlogo({color = undefined}: {color?: string}) {
  const textStyle = color ? {fill: color} : undefined;
  return (
    <>
      {/* Icon: 18×18 viewBox scaled to 36×36, centred vertically in the 64px box */}
      <g transform="translate(59,14) scale(2)">
        <defs>
          <linearGradient id="sts-apim-bg" x1="9" y1="16.82" x2="9" y2="1.18" gradientUnits="userSpaceOnUse">
            <stop offset="0" stopColor="#198ab3" />
            <stop offset="0.09" stopColor="#1f9dc4" />
            <stop offset="0.24" stopColor="#28b5d9" />
            <stop offset="0.4" stopColor="#2dc6e9" />
            <stop offset="0.57" stopColor="#31d1f2" />
            <stop offset="0.78" stopColor="#32d4f5" />
          </linearGradient>
          <linearGradient id="sts-apim-pill" x1="8.36" y1="11.35" x2="8.36" y2="14.46" gradientUnits="userSpaceOnUse">
            <stop offset="0" stopColor="#c69aeb" />
            <stop offset="1" stopColor="#6f4bb2" />
          </linearGradient>
        </defs>
        <path d="M14.18,5.89A4.85,4.85,0,0,0,9.23,1.18,5,5,0,0,0,4.48,4.47,4.61,4.61,0,0,0,.5,9,4.67,4.67,0,0,0,5.29,13.5a3,3,0,0,0,.42,0h1.2a1.47,1.47,0,0,1-.11-.56v0A1.51,1.51,0,0,1,7,12.21H5.6l-.31,0A3.41,3.41,0,0,1,1.77,9,3.33,3.33,0,0,1,4.68,5.73l.76-.12.25-.73A3.73,3.73,0,0,1,9.23,2.45,3.6,3.6,0,0,1,12.91,5.9V7L14,7.15a2.59,2.59,0,0,1,2.26,2.49,2.63,2.63,0,0,1-2.62,2.54h-.15l-.08,0h-1A3.92,3.92,0,0,0,8.54,9a.64.64,0,1,0,0,1.27,2.65,2.65,0,0,1,0,5.29.64.64,0,1,0,0,1.27,3.92,3.92,0,0,0,3.87-3.34h1.05a.64.64,0,0,0,.2,0A3.91,3.91,0,0,0,17.5,9.64,3.86,3.86,0,0,0,14.18,5.89Z" fill="url(#sts-apim-bg)" />
        <rect x="6.8" y="11.35" width="3.12" height="3.12" rx="1.54" fill="url(#sts-apim-pill)" />
      </g>
      <text x="103" y="26" className="sts-diagram__box-title" style={textStyle}>Azure API</text>
      <text x="103" y="42" className="sts-diagram__box-title" style={textStyle}>Management</text>
    </>
  );
}
