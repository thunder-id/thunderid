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
import {Box, Typography} from '@wso2/oxygen-ui';
import React from 'react';
import {APISIXLogo, AzureAPIMlogo, EnvoyLogo, KongLogo, KrakenDLogo} from './GatewayIntegrationDiagram';

interface GatewayConfig {
  logo: React.ReactNode;
  bgColor: string;
}

const GATEWAY_CONFIG: Record<string, GatewayConfig> = {
  'guides/integrations/apim-gateways/apisix':     {bgColor: '#1a0a14', logo: <APISIXLogo color="#fff" />},
  'guides/integrations/apim-gateways/azure-apim': {bgColor: '#002050', logo: <AzureAPIMlogo color="#fff" />},
  'guides/integrations/apim-gateways/envoy':      {bgColor: '#1a0a2e', logo: <EnvoyLogo color="#fff" />},
  'guides/integrations/apim-gateways/kong':       {bgColor: '#043558', logo: <KongLogo color="#fff" />},
  'guides/integrations/apim-gateways/krakend':    {bgColor: '#041c36', logo: <KrakenDLogo color="#fff" />},
};

const DEFAULT_BG = '#1e2a4a';

export function APIMGatewayTiles(): React.ReactElement {
  const category = useCurrentSidebarCategory();

  const items = category.items.filter(
    (item): item is PropSidebarItemLink =>
      item.type === 'link' && item.docId !== 'guides/integrations/apim-gateways/overview',
  );

  return (
    <Box
      sx={{
        display: 'grid',
        gap: '1rem',
        gridTemplateColumns: {xs: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)'},
        margin: '1rem 0 1.5rem',
      }}
    >
      {items.map(item => {
        const config = GATEWAY_CONFIG[item.docId ?? ''];
        return (
          <Box
            key={item.href}
            component="a"
            href={item.href}
            sx={{
              border: '1px solid',
              borderColor: 'var(--ifm-color-emphasis-200)',
              borderRadius: '12px',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
              textDecoration: 'none !important',
              transition: 'border-color 160ms ease, box-shadow 160ms ease, transform 160ms ease',
              '&:hover': {
                borderColor: 'color-mix(in srgb, var(--ifm-color-primary) 60%, transparent)',
                boxShadow: '0 4px 16px rgba(0,0,0,0.14)',
                textDecoration: 'none !important',
                transform: 'translateY(-2px)',
              },
            }}
          >
            <Box
              sx={{
                alignItems: 'center',
                background: config?.bgColor ?? DEFAULT_BG,
                display: 'flex',
                justifyContent: 'center',
                padding: '1.5rem 1.25rem',
                '& svg': {display: 'block', height: 'auto', maxWidth: '100%'},
              }}
            >
              <svg viewBox="0 0 240 64" xmlns="http://www.w3.org/2000/svg" width="200">
                {config?.logo ?? (
                  <text x="120" y="38" textAnchor="middle" fill="#fff" fontSize="16" fontWeight="bold">
                    {item.label}
                  </text>
                )}
              </svg>
            </Box>
            <Typography
              component="span"
              sx={{
                background: 'color-mix(in srgb, var(--ifm-color-emphasis-100) 30%, transparent)',
                borderTop: '1px solid',
                borderColor: 'var(--ifm-color-emphasis-200)',
                color: 'var(--ifm-font-color-base)',
                fontSize: '0.82rem',
                fontWeight: 600,
                padding: '0.55rem 1rem',
                textAlign: 'center',
              }}
            >
              {item.label}
            </Typography>
          </Box>
        );
      })}
    </Box>
  );
}
