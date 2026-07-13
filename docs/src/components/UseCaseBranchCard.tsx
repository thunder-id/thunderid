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
import {Box, Typography} from '@wso2/oxygen-ui';
import React from 'react';

interface UseCaseBranchCardProps {
  href: string;
  animationDelay: number;
  icon: React.ReactNode;
  accentColor: string;
  iconBackground: string;
  category: string;
  title: string;
  description: string;
  bullets: string[];
}

export default function UseCaseBranchCard({
  href,
  animationDelay,
  icon,
  accentColor,
  iconBackground,
  category,
  title,
  description,
  bullets,
}: UseCaseBranchCardProps) {
  return (
    <Box
      component={Link}
      to={href}
      sx={{
        '@keyframes ucEnterUp': {
          from: {opacity: 0, transform: 'translateY(18px)'},
          to: {opacity: 1, transform: 'translateY(0)'},
        },
        '&:hover': {
          boxShadow: '0 12px 28px rgba(0,0,0,0.12) !important',
          transform: 'translateY(-6px) scale(1.01)',
        },
        alignItems: 'flex-start',
        animation: 'ucEnterUp 700ms cubic-bezier(0.16,1,0.3,1) both',
        animationDelay: `${animationDelay}ms`,
        background: 'var(--ifm-background-surface)',
        border: '1px solid var(--ifm-color-emphasis-200)',
        borderRadius: '14px',
        boxShadow: '0 1px 6px rgba(0,0,0,0.05)',
        color: 'inherit',
        display: 'flex',
        flexDirection: 'column',
        flex: '1 1 0',
        maxWidth: 'none',
        minWidth: 0,
        padding: '1.75rem 1.5rem',
        textDecoration: 'none',
        width: '100%',
        transition: 'box-shadow 0.2s, transform 0.2s',
        willChange: 'transform, box-shadow',
      }}
    >
      <Box
        sx={{
          alignItems: 'center',
          background: iconBackground,
          borderRadius: '10px',
          color: accentColor,
          display: 'flex',
          flexShrink: 0,
          height: '48px',
          justifyContent: 'center',
          marginBottom: '1rem',
          width: '48px',
        }}
      >
        {icon}
      </Box>

      <Typography
        sx={{
          color: accentColor,
          fontSize: '0.68rem',
          fontWeight: 700,
          letterSpacing: '0.09em',
          marginBottom: '0.35rem',
          textTransform: 'uppercase',
        }}
      >
        {category}
      </Typography>

      <Typography
        sx={{
          color: 'var(--ifm-font-color-base)',
          fontSize: '1rem',
          fontWeight: 700,
          lineHeight: 1.3,
          marginBottom: '0.5rem',
        }}
      >
        {title}
      </Typography>

      <Typography
        sx={{
          color: 'var(--ifm-color-emphasis-700)',
          fontSize: '0.875rem',
          lineHeight: 1.6,
          marginBottom: '0.875rem',
        }}
      >
        {description}
      </Typography>

      <Box
        sx={{
          borderTop: '1px solid var(--ifm-color-emphasis-200)',
          flexGrow: 1,
          marginBottom: '1.25rem',
          paddingTop: '0.75rem',
        }}
      >
        <Typography
          sx={{
            color: 'var(--ifm-color-emphasis-500)',
            fontSize: '0.7rem',
            fontWeight: 700,
            letterSpacing: '0.08em',
            marginBottom: '0.4rem',
            textTransform: 'uppercase',
          }}
        >
          Choose when
        </Typography>

        <Box
          component="ul"
          sx={{
            color: 'var(--ifm-color-emphasis-700)',
            fontSize: '0.8rem',
            lineHeight: 1.6,
            listStyle: 'disc',
            margin: 0,
            padding: '0 0 0 1rem',
          }}
        >
          {bullets.map((bullet) => (
            <li key={bullet}>{bullet}</li>
          ))}
        </Box>
      </Box>

      <Typography
        sx={{
          color: accentColor,
          fontSize: '0.85rem',
          fontWeight: 600,
        }}
      >
        View pattern -&gt;
      </Typography>
    </Box>
  );
}
