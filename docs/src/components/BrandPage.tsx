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
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Container, Typography, useTheme} from '@wso2/oxygen-ui';
import {GitHub} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import ColorSchemeImage from '@site/src/components/ColorSchemeImage';
import useIsDarkMode from '@site/src/hooks/useIsDarkMode';

function CheckCircleIcon(): JSX.Element {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{color: '#22c55e', flexShrink: 0, marginTop: '2px'}}
    >
      <circle cx="12" cy="12" r="10" />
      <polyline points="9 12 11 14 15 10" />
    </svg>
  );
}

function XCircleIcon(): JSX.Element {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      style={{color: '#ef4444', flexShrink: 0, marginTop: '2px'}}
    >
      <circle cx="12" cy="12" r="10" />
      <line x1="15" y1="9" x2="9" y2="15" />
      <line x1="9" y1="9" x2="15" y2="15" />
    </svg>
  );
}

/* ─── Types ──────────────────────────────────────────────────────────────── */

interface AssetVariant {
  bg: string;
  src: string;
  alt: string;
  label: string;
  name: string;
}

interface RuleItem {
  text: string;
  type: 'do' | 'dont';
}

interface ColorSwatch {
  hex: string;
  rgb: string;
  name: string;
  desc: string;
}

/* ─── Download buttons ───────────────────────────────────────────────────── */

function DownloadButtons({svgPath, name}: {svgPath: string; name: string}): JSX.Element {
  const theme = useTheme();
  const base = svgPath.replace(/\.svg$/, '');
  const formats = ['SVG', 'PNG', 'WEBP'] as const;
  const paths: Record<(typeof formats)[number], string> = {
    SVG: svgPath,
    PNG: `${base}.png`,
    WEBP: `${base}.webp`,
  };
  return (
    <Box sx={{display: 'flex', gap: 0.75, mt: 1.25, flexWrap: 'wrap', justifyContent: 'center'}}>
      {formats.map((fmt, i) => (
        <Box
          key={fmt}
          component="a"
          href={paths[fmt]}
          download={`${name}.${fmt.toLowerCase()}`}
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 0.4,
            px: 1.25,
            py: 0.4,
            borderRadius: '6px',
            fontSize: '0.72rem',
            fontWeight: 700,
            letterSpacing: '0.03em',
            textDecoration: 'none',
            transition: 'all 0.15s ease',
            ...(i === 0
              ? {
                  bgcolor: 'primary.main',
                  color: '#fff',
                  '&:hover': {bgcolor: 'primary.dark', color: '#fff', textDecoration: 'none'},
                }
              : {
                  bgcolor: 'transparent',
                  color: 'text.secondary',
                  border: '1px solid',
                  borderColor: 'divider',
                  '&:hover': {
                    borderColor: theme.vars?.palette.primary.main,
                    color: 'primary.main',
                    textDecoration: 'none',
                  },
                }),
          }}
        >
          ↓ {fmt}
        </Box>
      ))}
    </Box>
  );
}

/* ─── Asset card ─────────────────────────────────────────────────────────── */

function AssetCard({variant}: {variant: AssetVariant}): JSX.Element {
  const isDark = useIsDarkMode();
  return (
    <Box
      sx={{
        flex: 1,
        minWidth: 200,
        borderRadius: '16px',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: isDark ? 'rgba(255,255,255,0.07)' : 'rgba(0,0,0,0.08)',
        boxShadow: isDark ? 'none' : '0 2px 12px rgba(0,0,0,0.05)',
        transition: 'box-shadow 0.2s ease, border-color 0.2s ease',
        '&:hover': {
          borderColor: isDark ? 'rgba(255,255,255,0.14)' : 'rgba(0,0,0,0.14)',
          boxShadow: isDark ? '0 0 0 1px rgba(255,255,255,0.06)' : '0 6px 24px rgba(0,0,0,0.09)',
        },
      }}
    >
      <Box
        sx={{
          bgcolor: variant.bg,
          py: 4,
          px: 3,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: 120,
        }}
      >
        <Box component="img" src={variant.src} alt={variant.alt} sx={{height: 70, maxWidth: '100%'}} />
      </Box>
      <Box
        sx={{
          px: 2,
          pt: 1.5,
          pb: 2,
          bgcolor: isDark ? 'rgba(255,255,255,0.025)' : 'rgba(255,255,255,0.9)',
          textAlign: 'center',
          borderTop: '1px solid',
          borderColor: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
        }}
      >
        <Typography sx={{fontSize: '0.78rem', fontWeight: 600, color: 'text.secondary'}}>{variant.label}</Typography>
        <DownloadButtons svgPath={variant.src} name={variant.name} />
      </Box>
    </Box>
  );
}

/* ─── Spec diagram ───────────────────────────────────────────────────────── */

function SpecDiagram({
  src,
  alt,
  imgHeight,
  clearSpaceLabel,
  minW,
  minH,
  context,
}: {
  src: {light: string; dark: string};
  alt: string;
  imgHeight: number;
  clearSpaceLabel: string;
  minW: string;
  minH: string;
  context: string;
}): JSX.Element {
  const isDark = useIsDarkMode();
  const theme = useTheme();
  const PAD = 36;

  return (
    <Box
      sx={{
        flex: 1,
        minWidth: 260,
        borderRadius: '16px',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: isDark ? 'rgba(255,255,255,0.07)' : 'rgba(0,0,0,0.08)',
        boxShadow: isDark ? 'none' : '0 2px 12px rgba(0,0,0,0.05)',
      }}
    >
      {/* Header pill */}
      <Box
        sx={{
          px: 2,
          py: 0.85,
          borderBottom: '1px solid',
          borderColor: isDark ? 'rgba(255,255,255,0.07)' : 'rgba(0,0,0,0.07)',
          bgcolor: isDark ? 'rgba(255,255,255,0.025)' : 'rgba(255,255,255,0.9)',
          display: 'flex',
          alignItems: 'center',
          gap: 1,
        }}
      >
        <Box
          sx={{
            width: 6,
            height: 6,
            borderRadius: '50%',
            bgcolor: 'primary.main',
            flexShrink: 0,
          }}
        />
        <Typography
          sx={{
            fontSize: '0.7rem',
            fontWeight: 700,
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: 'primary.main',
          }}
        >
          {context}
        </Typography>
      </Box>

      {/* Diagram canvas */}
      <Box
        sx={{
          position: 'relative',
          py: `${PAD + 28}px`,
          px: `${PAD + 44}px`,
          bgcolor: isDark ? 'rgba(255,255,255,0.02)' : 'rgba(248,250,252,1)',
          backgroundImage: isDark
            ? 'repeating-linear-gradient(45deg,rgba(255,255,255,0.03) 0,rgba(255,255,255,0.03) 1px,transparent 0,transparent 50%) 0 0/12px 12px'
            : 'repeating-linear-gradient(45deg,rgba(0,0,0,0.04) 0,rgba(0,0,0,0.04) 1px,transparent 0,transparent 50%) 0 0/12px 12px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        {/* Clear-space indicator */}
        <Box
          sx={{
            position: 'relative',
            p: `${PAD}px`,
            border: `1.5px dashed`,
            borderColor: theme.vars?.palette.primary.main ?? 'primary.main',
            borderRadius: '4px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            bgcolor: isDark ? 'rgba(5,33,63,0.5)' : '#fff',
          }}
        >
          <ColorSchemeImage src={src} alt={alt} height={imgHeight} style={{display: 'block', maxWidth: '100%'}} />
          {/* Clear space label */}
          <Typography
            sx={{
              position: 'absolute',
              top: '5px',
              left: '50%',
              transform: 'translateX(-50%)',
              fontSize: '0.6rem',
              fontWeight: 700,
              color: 'primary.main',
              bgcolor: isDark ? 'rgba(5,33,63,0.8)' : '#fff',
              px: 0.5,
              whiteSpace: 'nowrap',
            }}
          >
            {clearSpaceLabel}
          </Typography>
        </Box>

        {/* Min-width label */}
        <Box
          sx={{
            position: 'absolute',
            bottom: '10px',
            left: '50%',
            transform: 'translateX(-50%)',
            display: 'flex',
            alignItems: 'center',
            gap: 0.5,
            whiteSpace: 'nowrap',
          }}
        >
          <Typography sx={{fontSize: '0.68rem', color: 'text.secondary', fontWeight: 600}}>←</Typography>
          <Box
            sx={{
              px: 0.75,
              py: '2px',
              borderRadius: '4px',
              border: '1px solid',
              borderColor: 'divider',
              bgcolor: isDark ? 'rgba(255,255,255,0.05)' : '#fff',
              fontSize: '0.68rem',
              fontWeight: 700,
              color: 'text.secondary',
              fontFamily: 'var(--ifm-font-family-monospace)',
            }}
            component="span"
          >
            min {minW}
          </Box>
          <Typography sx={{fontSize: '0.68rem', color: 'text.secondary', fontWeight: 600}}>→</Typography>
        </Box>

        {/* Min-height label */}
        <Box
          sx={{
            position: 'absolute',
            right: '6px',
            top: '50%',
            transform: 'translateY(-50%) rotate(90deg)',
            display: 'flex',
            alignItems: 'center',
            gap: 0.5,
            whiteSpace: 'nowrap',
          }}
        >
          <Typography sx={{fontSize: '0.68rem', color: 'text.secondary', fontWeight: 600}}>←</Typography>
          <Box
            component="span"
            sx={{
              px: 0.75,
              py: '2px',
              borderRadius: '4px',
              border: '1px solid',
              borderColor: 'divider',
              bgcolor: isDark ? 'rgba(255,255,255,0.05)' : '#fff',
              fontSize: '0.68rem',
              fontWeight: 700,
              color: 'text.secondary',
              fontFamily: 'var(--ifm-font-family-monospace)',
            }}
          >
            min {minH}
          </Box>
          <Typography sx={{fontSize: '0.68rem', color: 'text.secondary', fontWeight: 600}}>→</Typography>
        </Box>
      </Box>

      {/* Legend */}
      <Box
        sx={{
          px: 2,
          py: 1,
          borderTop: '1px solid',
          borderColor: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
          bgcolor: isDark ? 'rgba(255,255,255,0.025)' : 'rgba(255,255,255,0.9)',
          display: 'flex',
          gap: 2.5,
          flexWrap: 'wrap',
        }}
      >
        {[
          {symbol: '╌╌', label: 'Clear space'},
          {symbol: '↔', label: 'Minimum size'},
        ].map(({symbol, label}) => (
          <Box key={label} sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
            <Typography sx={{fontSize: '0.72rem', color: 'primary.main', fontWeight: 700}}>{symbol}</Typography>
            <Typography sx={{fontSize: '0.72rem', color: 'text.secondary'}}>{label}</Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/* ─── Usage rule list ────────────────────────────────────────────────────── */

function RuleList({items}: {items: RuleItem[]}): JSX.Element {
  const isDark = useIsDarkMode();
  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 0.75}}>
      {items.map(({text, type}) => (
        <Box
          key={text}
          sx={{
            display: 'flex',
            gap: 1,
            alignItems: 'flex-start',
            px: 1.5,
            py: 1,
            borderRadius: '10px',
            border: '1px solid',
            fontSize: '0.84rem',
            lineHeight: 1.5,
            ...(type === 'do'
              ? {
                  bgcolor: isDark ? 'rgba(34,197,94,0.08)' : 'rgba(34,197,94,0.06)',
                  borderColor: isDark ? 'rgba(34,197,94,0.2)' : 'rgba(34,197,94,0.2)',
                }
              : {
                  bgcolor: isDark ? 'rgba(239,68,68,0.08)' : 'rgba(239,68,68,0.06)',
                  borderColor: isDark ? 'rgba(239,68,68,0.2)' : 'rgba(239,68,68,0.2)',
                }),
          }}
        >
          <Typography component="span" sx={{flexShrink: 0, fontSize: '0.85rem'}}>
            {type === 'do' ? <CheckCircleIcon /> : <XCircleIcon />}
          </Typography>
          <Typography sx={{fontSize: '0.84rem', color: 'text.secondary', lineHeight: 1.5}}>{text}</Typography>
        </Box>
      ))}
    </Box>
  );
}

/* ─── Section heading ────────────────────────────────────────────────────── */

function SectionHeading({overline, title}: {overline: string; title: string}): JSX.Element {
  return (
    <Box sx={{mb: {xs: 4, md: 5}}}>
      <Typography
        variant="overline"
        sx={{
          display: 'block',
          fontSize: '0.7rem',
          letterSpacing: '0.18em',
          color: 'primary.main',
          fontWeight: 600,
          mb: 0.5,
        }}
      >
        {overline}
      </Typography>
      <Typography
        variant="h3"
        sx={{
          fontWeight: 700,
          fontSize: {xs: '1.5rem', md: '1.75rem'},
          letterSpacing: '-0.02em',
          color: 'text.primary',
        }}
      >
        {title}
      </Typography>
    </Box>
  );
}

/* ─── Divider ────────────────────────────────────────────────────────────── */

function Divider(): JSX.Element {
  return (
    <Box
      sx={{
        height: '1px',
        bgcolor: 'divider',
        my: {xs: 6, md: 8},
      }}
    />
  );
}

/* ─── Color swatch card ──────────────────────────────────────────────────── */

function ColorCard({swatch}: {swatch: ColorSwatch}): JSX.Element {
  const isDark = useIsDarkMode();
  return (
    <Box
      sx={{
        flex: 1,
        minWidth: 180,
        borderRadius: '16px',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: isDark ? 'rgba(255,255,255,0.07)' : 'rgba(0,0,0,0.08)',
        boxShadow: isDark ? 'none' : '0 2px 12px rgba(0,0,0,0.05)',
        transition: 'box-shadow 0.2s ease',
        '&:hover': {
          boxShadow: isDark ? 'none' : '0 6px 24px rgba(0,0,0,0.09)',
        },
      }}
    >
      <Box
        sx={{
          height: 80,
          bgcolor: swatch.hex,
          border: swatch.hex === '#FFFFFF' ? '1px solid rgba(0,0,0,0.08)' : 'none',
        }}
      />
      <Box
        sx={{
          p: 1.75,
          bgcolor: isDark ? 'rgba(255,255,255,0.025)' : 'rgba(255,255,255,0.9)',
          borderTop: '1px solid',
          borderColor: isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)',
        }}
      >
        <Typography sx={{fontWeight: 700, fontSize: '0.875rem', color: 'text.primary', mb: 0.25}}>
          {swatch.name}
        </Typography>
        <Typography
          sx={{
            fontSize: '0.8rem',
            fontFamily: 'var(--ifm-font-family-monospace)',
            color: 'primary.main',
            mb: 0.25,
          }}
        >
          {swatch.hex}
        </Typography>
        <Typography sx={{fontSize: '0.73rem', color: 'text.disabled', mb: 0.75}}>RGB {swatch.rgb}</Typography>
        <Typography sx={{fontSize: '0.78rem', color: 'text.secondary'}}>{swatch.desc}</Typography>
      </Box>
    </Box>
  );
}

/* ─── Main component ─────────────────────────────────────────────────────── */

const LOGO_VARIANTS: AssetVariant[] = [
  {
    bg: '#ffffff',
    src: '/assets/images/logo.svg',
    alt: 'ThunderID logo — light background',
    label: 'Dark on light',
    name: 'thunderid-logo',
  },
  {
    bg: '#05213F',
    src: '/assets/images/logo-inverted.svg',
    alt: 'ThunderID logo — dark background',
    label: 'Light on dark',
    name: 'thunderid-logo-inverted',
  },
];

const ICON_VARIANTS: AssetVariant[] = [
  {
    bg: '#ffffff',
    src: '/assets/images/logo-mini.svg',
    alt: 'ThunderID icon — light background',
    label: 'Dark on light',
    name: 'thunderid-icon',
  },
  {
    bg: '#05213F',
    src: '/assets/images/logo-mini-inverted.svg',
    alt: 'ThunderID icon — dark background',
    label: 'Light on dark',
    name: 'thunderid-icon-inverted',
  },
];

const LOGO_DO: RuleItem[] = [
  {type: 'do', text: 'Use the dark logo on white or light-coloured backgrounds.'},
  {type: 'do', text: 'Use the light (inverted) logo on dark or coloured backgrounds.'},
  {type: 'do', text: 'Use SVG at any scale — it remains sharp at all sizes.'},
  {type: 'do', text: 'Maintain clear space equal to the wordmark cap-height on all sides.'},
];

const LOGO_DONT: RuleItem[] = [
  {type: 'dont', text: 'Do not recolour, tint, or apply effects to the logo.'},
  {type: 'dont', text: 'Do not place an outline or drop shadow around the logo.'},
  {type: 'dont', text: 'Do not stretch, compress, skew, or rotate the logo.'},
  {type: 'dont', text: 'Do not scale the icon and wordmark independently.'},
  {type: 'dont', text: 'Do not place the logo on a busy or low-contrast background.'},
];

const ICON_DO: RuleItem[] = [
  {type: 'do', text: 'Dark icon on white or light backgrounds.'},
  {type: 'do', text: 'Light icon on dark or coloured backgrounds.'},
  {type: 'do', text: 'Light icon on the Electric Blue (#3688FF) brand background.'},
  {type: 'do', text: 'Use in place of the full logo only when context makes the brand clear (e.g. favicon, avatar).'},
];

const ICON_DONT: RuleItem[] = [
  {type: 'dont', text: 'Do not recolour or apply gradients to the icon.'},
  {type: 'dont', text: 'Do not warp, rotate, or distort the icon.'},
  {type: 'dont', text: 'Do not use the icon in isolation in marketing material where the full logo should appear.'},
  {type: 'dont', text: 'Do not combine the icon with a different wordmark.'},
];

const COLORS: ColorSwatch[] = [
  {hex: '#05213F', rgb: '5, 33, 63', name: 'Deep Navy', desc: 'Primary brand color — wordmark and dark backgrounds'},
  {hex: '#3688FF', rgb: '54, 136, 255', name: 'Electric Blue', desc: 'Accent — icon highlight, links, call-to-action'},
  {hex: '#FFFFFF', rgb: '255, 255, 255', name: 'White', desc: 'Light backgrounds and inverted text'},
];

const NAME_DO: RuleItem[] = [{type: 'do', text: 'ThunderID'}];
const NAME_DONT: RuleItem[] = [
  {type: 'dont', text: 'Thunder ID'},
  {type: 'dont', text: 'Thunderid'},
  {type: 'dont', text: 'thunderid'},
  {type: 'dont', text: 'Thunder-ID'},
];

export default function BrandPage(): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;
  const repoUrl = (siteConfig.customFields?.product as DocusaurusProductConfig).project.source.github.url;

  return (
    <Box sx={{py: {xs: 6, lg: 8}}}>
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}}}>
        {/* Page heading */}
        <Box sx={{mb: {xs: 6, md: 8}}}>
          <Typography
            variant="overline"
            sx={{
              display: 'block',
              fontSize: '0.7rem',
              letterSpacing: '0.18em',
              color: 'primary.main',
              fontWeight: 600,
              mb: 0.75,
            }}
          >
            Visual Identity
          </Typography>
          <Typography
            variant="h1"
            sx={{
              fontWeight: 800,
              fontSize: {xs: '2rem', md: '2.75rem'},
              letterSpacing: '-0.03em',
              color: 'text.primary',
              mb: 1.5,
            }}
          >
            Brand Guidelines
          </Typography>
          <Typography
            sx={{fontSize: {xs: '1rem', md: '1.1rem'}, color: 'text.secondary', maxWidth: '640px', lineHeight: 1.7}}
          >
            How to use the {productName} name, logo, and visual assets correctly. Please follow these guidelines when
            representing {productName} in press, community content, integrations, or open-source contributions.
          </Typography>
        </Box>

        {/* ── Logo ── */}
        <SectionHeading overline="Primary mark" title="Logo" />

        <Box sx={{display: 'flex', gap: 2, flexWrap: 'wrap', mb: 4}}>
          {LOGO_VARIANTS.map((v) => (
            <AssetCard key={v.name} variant={v} />
          ))}
        </Box>

        <Box sx={{mb: 2}}>
          <Typography
            sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary', mb: 2, letterSpacing: '-0.01em'}}
          >
            Technical specifications
          </Typography>
          <Box sx={{display: 'flex', gap: 2, flexWrap: 'wrap'}}>
            <SpecDiagram
              src={{dark: '/assets/images/logo-inverted.svg', light: '/assets/images/logo.svg'}}
              alt="Logo digital spec"
              imgHeight={36}
              clearSpaceLabel="cap-height on all sides"
              minW="180px"
              minH="48px"
              context="Digital"
            />
            <SpecDiagram
              src={{dark: '/assets/images/logo-inverted.svg', light: '/assets/images/logo.svg'}}
              alt="Logo print spec"
              imgHeight={36}
              clearSpaceLabel="cap-height on all sides"
              minW="50mm"
              minH="13mm"
              context="Print"
            />
          </Box>
        </Box>

        <Box sx={{mt: 4, display: 'none'}}>
          <Typography
            sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary', mb: 2, letterSpacing: '-0.01em'}}
          >
            Usage guide
          </Typography>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 2}}>
            <Box>
              <Typography
                sx={{
                  fontWeight: 600,
                  fontSize: '0.8rem',
                  color: 'text.secondary',
                  mb: 1,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                }}
              >
                Approved use
              </Typography>
              <RuleList items={LOGO_DO} />
            </Box>
            <Box>
              <Typography
                sx={{
                  fontWeight: 600,
                  fontSize: '0.8rem',
                  color: 'text.secondary',
                  mb: 1,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                }}
              >
                Prohibited use
              </Typography>
              <RuleList items={LOGO_DONT} />
            </Box>
          </Box>
        </Box>

        <Divider />

        {/* ── Icon ── */}
        <SectionHeading overline="Compact mark" title="Icon (mark only)" />

        <Box sx={{display: 'flex', gap: 2, flexWrap: 'wrap', mb: 4}}>
          {ICON_VARIANTS.map((v) => (
            <AssetCard key={v.name} variant={v} />
          ))}
        </Box>

        <Box sx={{mb: 2}}>
          <Typography
            sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary', mb: 2, letterSpacing: '-0.01em'}}
          >
            Technical specifications
          </Typography>
          <Box sx={{display: 'flex', gap: 2, flexWrap: 'wrap'}}>
            <SpecDiagram
              src={{dark: '/assets/images/logo-mini-inverted.svg', light: '/assets/images/logo-mini.svg'}}
              alt="Icon digital spec"
              imgHeight={44}
              clearSpaceLabel="25% of icon width"
              minW="32px"
              minH="32px"
              context="Digital"
            />
            <SpecDiagram
              src={{dark: '/assets/images/logo-mini-inverted.svg', light: '/assets/images/logo-mini.svg'}}
              alt="Icon print spec"
              imgHeight={44}
              clearSpaceLabel="25% of icon width"
              minW="10mm"
              minH="10mm"
              context="Print"
            />
          </Box>
        </Box>

        <Box sx={{mt: 4, display: 'none'}}>
          <Typography
            sx={{fontWeight: 700, fontSize: '0.9rem', color: 'text.primary', mb: 2, letterSpacing: '-0.01em'}}
          >
            Usage guide
          </Typography>
          <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 2}}>
            <Box>
              <Typography
                sx={{
                  fontWeight: 600,
                  fontSize: '0.8rem',
                  color: 'text.secondary',
                  mb: 1,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                }}
              >
                Approved use
              </Typography>
              <RuleList items={ICON_DO} />
            </Box>
            <Box>
              <Typography
                sx={{
                  fontWeight: 600,
                  fontSize: '0.8rem',
                  color: 'text.secondary',
                  mb: 1,
                  textTransform: 'uppercase',
                  letterSpacing: '0.08em',
                }}
              >
                Prohibited use
              </Typography>
              <RuleList items={ICON_DONT} />
            </Box>
          </Box>
        </Box>

        <Divider />

        {/* ── Colors ── */}
        <SectionHeading overline="Visual identity" title="Colors" />
        <Box sx={{display: 'flex', gap: 2, flexWrap: 'wrap', mb: 2}}>
          {COLORS.map((c) => (
            <ColorCard key={c.hex} swatch={c} />
          ))}
        </Box>

        <Divider />

        {/* ── Name usage ── */}
        <SectionHeading overline="Nomenclature" title="Name usage" />

        <Typography sx={{color: 'text.secondary', mb: 3, fontSize: '0.95rem', lineHeight: 1.7, maxWidth: '600px'}}>
          When referring to the project in text, use <strong>ThunderID</strong> — one word, capital T and capital ID.
        </Typography>

        <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 2, mb: 3}}>
          <Box>
            <Typography
              sx={{
                fontWeight: 600,
                fontSize: '0.8rem',
                color: 'text.secondary',
                mb: 1,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
            >
              Correct
            </Typography>
            <RuleList items={NAME_DO} />
          </Box>
          <Box>
            <Typography
              sx={{
                fontWeight: 600,
                fontSize: '0.8rem',
                color: 'text.secondary',
                mb: 1,
                textTransform: 'uppercase',
                letterSpacing: '0.08em',
              }}
            >
              Incorrect
            </Typography>
            <RuleList items={NAME_DONT} />
          </Box>
        </Box>

        <Typography sx={{color: 'text.secondary', fontSize: '0.875rem', lineHeight: 1.6}}>
          Do not use &ldquo;{productName}&rdquo; in your own product or service name in a way that implies an official
          relationship.
        </Typography>

        <Divider />

        {/* ── Questions ── */}
        <Box
          sx={{
            p: {xs: 3, md: 4},
            borderRadius: '16px',
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
            display: 'flex',
            flexDirection: {xs: 'column', sm: 'row'},
            alignItems: {xs: 'flex-start', sm: 'center'},
            justifyContent: 'space-between',
            gap: 2,
          }}
        >
          <Box>
            <Typography sx={{fontWeight: 700, fontSize: '1rem', color: 'text.primary', mb: 0.5}}>
              Questions about brand use?
            </Typography>
            <Typography sx={{color: 'text.secondary', fontSize: '0.875rem'}}>
              For permission requests, open a discussion on GitHub.
            </Typography>
          </Box>
          <Box
            component={Link}
            href={`${repoUrl}/discussions`}
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 1,
              px: 2.5,
              py: 1.1,
              borderRadius: '10px',
              bgcolor: 'primary.main',
              color: '#fff',
              fontWeight: 700,
              fontSize: '0.875rem',
              textDecoration: 'none',
              flexShrink: 0,
              transition: 'background-color 0.15s ease',
              '&:hover': {bgcolor: 'primary.dark', color: '#fff', textDecoration: 'none'},
              '& svg': {display: 'block', width: '1.1rem', height: '1.1rem'},
            }}
          >
            <GitHub />
            GitHub Discussions
          </Box>
        </Box>
      </Container>
    </Box>
  );
}
