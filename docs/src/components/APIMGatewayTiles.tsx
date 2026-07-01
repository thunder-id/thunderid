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

import type {PropSidebarItemLink} from '@docusaurus/plugin-content-docs';
import {useCurrentSidebarCategory} from '@docusaurus/plugin-content-docs/client';
import React from 'react';
import {APISIXLogo, AzureAPIMlogo, EnvoyLogo, KongLogo, KrakenDLogo} from './GatewayIntegrationDiagram';

interface GatewayConfig {
  logo: React.ReactNode;
  bgColor: string;
}

const GATEWAY_CONFIG: Record<string, GatewayConfig> = {
  'guides/guides/integrations/apim-gateways/apisix': {
    bgColor: '#1a0a14',
    logo: <APISIXLogo color="#fff" />,
  },
  'guides/guides/integrations/apim-gateways/azure-apim': {
    bgColor: '#002050',
    logo: <AzureAPIMlogo color="#fff" />,
  },
  'guides/guides/integrations/apim-gateways/envoy': {
    bgColor: '#1a0a2e',
    logo: <EnvoyLogo color="#fff" />,
  },
  'guides/guides/integrations/apim-gateways/kong': {
    bgColor: '#043558',
    logo: <KongLogo color="#fff" />,
  },
  'guides/guides/integrations/apim-gateways/krakend': {
    bgColor: '#041c36',
    logo: <KrakenDLogo color="#fff" />,
  },
};

const DEFAULT_BG = '#1e2a4a';

export function APIMGatewayTiles() {
  const category = useCurrentSidebarCategory();

  const items = category.items.filter(
    (item): item is PropSidebarItemLink =>
      item.type === 'link' && item.docId !== 'guides/guides/integrations/apim-gateways/overview',
  );

  return (
    <div className="gw-tile-grid">
      {items.map(item => {
        const config = GATEWAY_CONFIG[item.docId ?? ''];
        return (
          <a key={item.href} href={item.href} className="gw-tile">
            <div className="gw-tile__logo" style={{background: config?.bgColor ?? DEFAULT_BG}}>
              <svg viewBox="0 0 240 64" xmlns="http://www.w3.org/2000/svg" width="200">
                {config?.logo ?? (
                  <text x="120" y="38" textAnchor="middle" fill="#fff" fontSize="16" fontWeight="bold">
                    {item.label}
                  </text>
                )}
              </svg>
            </div>
            <span className="gw-tile__name">{item.label}</span>
          </a>
        );
      })}
    </div>
  );
}
