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

import Link from '@docusaurus/Link';
import {Box, Chip, Typography, useTheme} from '@wso2/oxygen-ui';
import {Bot, MonitorSmartphone, Server} from '@wso2/oxygen-ui-icons-react';
import React from 'react';

interface IntegrationType {
  icon: React.ReactElement;
  title: string;
  description: string;
  href?: string;
  comingSoon?: boolean;
}

const TYPES: IntegrationType[] = [
  {
    icon: <MonitorSmartphone size={28} />,
    title: 'Application',
    description: 'Web, mobile, and desktop apps. React, Vue, Next.js, iOS, Android, and more.',
    href: '/docs/next/getting-started/connect-your-application',
  },
  {
    icon: <Bot size={28} />,
    title: 'AI Agent',
    description: 'Add identity and authorization to LLM-powered agents.',
    comingSoon: true,
  },
  {
    icon: <Server size={28} />,
    title: 'MCP Server',
    description: 'Secure Model Context Protocol servers with built-in auth.',
    comingSoon: true,
  },
];

export default function IntegrationTypePicker(): React.ReactElement {
  const theme = useTheme();

  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2,
        gridTemplateColumns: {xs: '1fr', sm: 'repeat(3, 1fr)'},
        my: 2,
      }}
    >
      {TYPES.map(({icon, title, description, href, comingSoon}) => {
        const card = (
          <Box
            sx={{
              border: '1px solid',
              borderColor: comingSoon ? 'divider' : `rgba(${theme.vars?.palette.primary.main} / 0.25)`,
              borderRadius: '14px',
              cursor: comingSoon ? 'default' : 'pointer',
              display: 'flex',
              flexDirection: 'column',
              gap: 1.5,
              opacity: comingSoon ? 0.55 : 1,
              p: 3,
              transition: 'border-color 0.15s, background-color 0.15s',
              ...(!comingSoon && {
                '&:hover': {
                  bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.04)`,
                  borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.5)`,
                },
              }),
            }}
          >
            <Box
              sx={{
                alignItems: 'center',
                bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.1)`,
                borderRadius: '10px',
                color: 'primary.main',
                display: 'inline-flex',
                height: 48,
                justifyContent: 'center',
                width: 48,
              }}
            >
              {icon}
            </Box>
            <Box>
              <Box sx={{alignItems: 'center', display: 'flex', gap: 1, mb: 0.5}}>
                <Typography sx={{color: 'text.primary', fontSize: '1rem', fontWeight: 700}}>
                  {title}
                </Typography>
                {comingSoon && (
                  <Chip label="Coming soon" size="small" sx={{fontSize: '0.7rem', height: 20}} />
                )}
              </Box>
              <Typography sx={{color: 'text.secondary', fontSize: '0.85rem', lineHeight: 1.6}}>
                {description}
              </Typography>
            </Box>
          </Box>
        );

        return comingSoon ? (
          <div key={title}>{card}</div>
        ) : (
          <Link key={title} to={href} style={{textDecoration: 'none', color: 'inherit'}}>
            {card}
          </Link>
        );
      })}
    </Box>
  );
}
