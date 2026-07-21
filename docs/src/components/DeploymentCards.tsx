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


interface Feature {
  text: string;
  available: boolean;
}

interface Station {
  href: string;
  accentColor: string;
  iconBackground: string;
  title: string;
  chooseIf: string;
  cta: string;
  icon: React.ReactNode;
  features: Feature[];
  featured?: boolean;
  animDelay: number;
}

const LOGO_STYLE: React.CSSProperties = {display: 'block', height: '26px', objectFit: 'contain', width: '26px'};

function DockerLogo() {
  return <img src="/assets/images/docker-logo.svg" alt="Docker" style={LOGO_STYLE} />;
}

function KubernetesLogo() {
  return <img src="/assets/images/kubernetes-logo.svg" alt="Kubernetes" style={LOGO_STYLE} />;
}

function OpenChoreoLogo() {
  return (
    <>
      <Box component="span" sx={{'[data-theme="dark"] &': {display: 'none'}}}>
        <img src="/assets/images/openchoreo-logo.svg" alt="OpenChoreo" style={LOGO_STYLE} />
      </Box>
      <Box component="span" sx={{display: 'none', '[data-theme="dark"] &': {display: 'inline'}}}>
        <img src="https://openchoreo.dev/img/openchoreo-logo-dark.svg" alt="OpenChoreo" style={LOGO_STYLE} />
      </Box>
    </>
  );
}

function CheckIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  );
}

function DashIcon() {
  return (
    <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3" strokeLinecap="round" strokeLinejoin="round">
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  );
}

function buildStations(): Station[] {
  return [
  {
    href: './docker',
    accentColor: '#0ea5e9',
    iconBackground: 'rgba(14,165,233,0.12)',
    title: 'Docker',
    chooseIf: 'You want a portable, self-contained deployment with full configuration control and no cluster infrastructure required.',
    cta: 'Continue →',
    icon: <DockerLogo />,
    features: [
      {text: 'Quick setup with pre-built images', available: true},
      {text: 'PostgreSQL integration', available: true},
      {text: 'Custom configuration mounting', available: true},
    ],
    animDelay: 450,
  },
  {
    href: './kubernetes',
    accentColor: '#326CE5',
    iconBackground: 'rgba(50,108,229,0.12)',
    title: 'Kubernetes',
    chooseIf: 'You want production control while running ThunderID on infrastructure your team manages.',
    cta: 'Continue →',
    icon: <KubernetesLogo />,
    features: [
      {text: 'Helm chart deployment', available: true},
      {text: 'Multi-replica support', available: true},
      {text: 'Ingress configuration', available: true},
      {text: 'Database flexibility (PostgreSQL/SQLite)', available: true},
      {text: 'Rolling updates and rollbacks', available: true},
    ],
    animDelay: 570,
  },
  {
    href: './openchoreo',
    accentColor: '#8b5cf6',
    iconBackground: 'rgba(139,92,246,0.12)',
    title: 'OpenChoreo',
    chooseIf: 'You want a platform-managed deployment model with environment separation and promotion workflows.',
    cta: 'Continue →',
    icon: <OpenChoreoLogo />,
    features: [
      {text: 'Cell-based deployment model', available: true},
      {text: 'Integrated platform services', available: true},
      {text: 'Advanced networking', available: true},
      {text: 'Service mesh integration', available: true},
    ],
    animDelay: 690,
  },
];}

export default function DeploymentCards(): React.ReactElement {
  const stations = buildStations();
  return (
    <Box
      sx={{
        margin: '2rem 0 3.5rem',
        '@keyframes dpFadeUp': {from: {opacity: 0, transform: 'translateY(18px)'}, to: {opacity: 1, transform: 'translateY(0)'}},
      }}
    >
      {/* Header */}
      <Box sx={{animation: 'dpFadeUp 700ms cubic-bezier(0.16,1,0.3,1) 0ms both', marginBottom: '2.5rem'}}>
        <Typography sx={{color: 'var(--ifm-color-primary)', fontSize: '0.7rem', fontWeight: 800, letterSpacing: '0.13em', marginBottom: '0.5rem', textTransform: 'uppercase'}}>
          Deployment
        </Typography>
        <Typography component="h1" sx={{color: 'var(--ifm-font-color-base)', fontSize: '2rem', fontWeight: 800, letterSpacing: '-0.03em', lineHeight: 1.15, margin: '0 0 0.55rem'}}>
          Where are you deploying?
        </Typography>
        <Typography sx={{color: 'var(--ifm-color-emphasis-600)', fontSize: '1rem', lineHeight: 1.65, margin: 0, maxWidth: '540px'}}>
          Pick the environment that matches your setup. Each option has a dedicated installation guide.
        </Typography>
      </Box>

      {/* Cards grid */}
      <Box
        sx={{
          alignItems: 'stretch',
          display: 'grid',
          gap: '1rem',
          gridTemplateColumns: {xs: '1fr', xl: 'repeat(3, 1fr)'},
          marginLeft: 0,
          marginRight: {md: '-2rem'},
        }}
      >
        {stations.map((s) => (
          <Box
            key={s.href}
            component={Link}
            to={s.href}
            sx={{
              animation: `dpFadeUp 700ms cubic-bezier(0.16,1,0.3,1) ${s.animDelay}ms both`,
              justifySelf: {xs: 'center', xl: 'stretch'},
              maxWidth: {xs: '540px', xl: 'none'},
              width: '100%',
              background: 'var(--ifm-background-surface-color)',
              '[data-theme="dark"] &': {background: 'rgba(255,255,255,0.04)'},
              border: '1px solid',
              borderColor: s.featured ? s.accentColor : 'var(--ifm-color-emphasis-200)',
              borderRadius: '14px',
              boxShadow: s.featured ? `0 0 0 1px ${s.accentColor}, 0 6px 28px color-mix(in srgb, ${s.accentColor} 20%, transparent)` : 'none',
              color: 'inherit',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
              padding: '2rem 2rem 1.75rem',
              position: 'relative',
              textDecoration: 'none !important',
              transition: 'border-color 220ms ease, box-shadow 220ms ease, transform 220ms ease',
              '&::before': {
                background: s.accentColor,
                borderRadius: '14px 14px 0 0',
                content: '""',
                height: '3px',
                inset: '0 0 auto 0',
                opacity: s.featured ? 1 : 0.7,
                position: 'absolute',
              },
              '&:hover': {
                borderColor: s.accentColor,
                boxShadow: `0 0 0 1px ${s.accentColor}, 0 10px 36px rgba(0,0,0,0.18)`,
                color: 'inherit',
                textDecoration: 'none !important',
                transform: 'translateY(-3px)',
              },
            }}
          >
            {/* Logo badge */}
            <Box
              sx={{
                alignItems: 'center',
                alignSelf: 'center',
                background: s.iconBackground,
                border: `2px solid ${s.accentColor}`,
                borderRadius: '50%',
                display: 'flex',
                flexShrink: 0,
                height: '52px',
                justifyContent: 'center',
                marginBottom: '1.25rem',
                width: '52px',
              }}
            >
              {s.icon}
            </Box>

            {/* Title */}
            <Typography sx={{color: 'var(--ifm-font-color-base)', fontSize: '1.9rem', fontWeight: 800, letterSpacing: '-0.035em', lineHeight: 1.05, marginBottom: '0.5rem', textAlign: 'center'}}>
              {s.title}
            </Typography>

            {/* Feature list */}
            <Box component="ul" sx={{display: 'flex', flexDirection: 'column', gap: '0.85rem', listStyle: 'none', margin: '0 0 1.35rem', padding: 0}}>
              {s.features.map((f) => (
                <Box
                  key={f.text}
                  component="li"
                  sx={{
                    alignItems: 'flex-start',
                    color: f.available ? 'var(--ifm-color-emphasis-800)' : 'var(--ifm-color-emphasis-400)',
                    display: 'flex',
                    fontSize: '0.82rem',
                    gap: '0.5rem',
                    lineHeight: 1.5,
                  }}
                >
                  <Box
                    component="span"
                    sx={{
                      alignItems: 'center',
                      color: f.available ? '#22c55e' : 'var(--ifm-color-emphasis-400)',
                      display: 'flex',
                      flexShrink: 0,
                      marginTop: '3px',
                    }}
                  >
                    {f.available ? <CheckIcon /> : <DashIcon />}
                  </Box>
                  <span>{f.text}</span>
                </Box>
              ))}
            </Box>

            {/* Choose if */}
            <Box sx={{borderTop: '1px solid', borderColor: 'var(--ifm-color-emphasis-200)', margin: 'auto 0 1.5rem', minHeight: '8.5rem', paddingTop: '1.25rem'}}>
              <Typography sx={{color: 'var(--ifm-color-emphasis-500)', fontSize: '0.7rem', fontWeight: 700, letterSpacing: '0.08em', marginBottom: '0.4rem', textTransform: 'uppercase'}}>
                Choose this if…
              </Typography>
              <Typography sx={{color: 'var(--ifm-color-emphasis-700)', fontSize: '0.8rem', lineHeight: 1.6, margin: 0}}>
                {s.chooseIf}
              </Typography>
            </Box>

            {/* CTA */}
            <Typography
              component="span"
              sx={{
                alignItems: 'center',
                background: s.featured ? s.accentColor : 'transparent',
                border: '1.5px solid',
                borderColor: s.featured ? 'transparent' : s.accentColor,
                borderRadius: '8px',
                boxShadow: s.featured ? `0 2px 10px color-mix(in srgb, ${s.accentColor} 35%, transparent)` : 'none',
                color: s.featured ? '#fff !important' : `${s.accentColor} !important`,
                display: 'flex',
                fontSize: '0.82rem',
                fontWeight: 700,
                justifyContent: 'center',
                letterSpacing: '0.01em',
                padding: '0.7rem 1.25rem',
                transition: 'background 180ms ease, color 180ms ease, transform 180ms ease',
                'a:hover &': {
                  background: s.accentColor,
                  borderColor: 'transparent',
                  color: '#fff !important',
                  transform: 'translateY(-1px)',
                },
              }}
            >
              {s.cta}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}
