// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {JSX, ReactNode, useEffect, useRef, useState} from 'react';
import {useInk} from './theme';
import type {EcosystemCode, EcosystemCta} from '@site/src/types/ecosystem';

/** Small uppercase pill used for statuses, tags, and table badges. */
export function Pill({label, colour, filled = false}: {label: string; colour: string; filled?: boolean}): JSX.Element {
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        flexShrink: 0,
        fontFamily: 'monospace',
        fontSize: '9px',
        fontWeight: 600,
        letterSpacing: '0.09em',
        textTransform: 'uppercase',
        borderRadius: '5px',
        px: '7px',
        py: '3px',
        whiteSpace: 'nowrap',
        color: colour,
        bgcolor: filled ? `color-mix(in srgb, ${colour} 12%, transparent)` : 'transparent',
        border: '1px solid',
        borderColor: `color-mix(in srgb, ${colour} 30%, transparent)`,
      }}
    >
      {label}
    </Box>
  );
}

/** Eyebrow above a block of stats or related links. */
export function RailHeading({children}: {children: ReactNode}): JSX.Element {
  const ink = useInk();
  return (
    <Typography
      component="div"
      sx={{
        fontFamily: 'monospace',
        fontSize: '9.5px',
        letterSpacing: '0.14em',
        textTransform: 'uppercase',
        color: ink(0.4, 0.3),
        mb: 1.5,
      }}
    >
      {children}
    </Typography>
  );
}

/** Section title, with the optional "~8 min" style label beside it. */
export function SectionHeading({
  id,
  title,
  meta = undefined,
  intro = undefined,
}: {
  id: string;
  title: string;
  meta?: string;
  intro?: string;
}): JSX.Element {
  const ink = useInk();
  return (
    <>
      <Box sx={{display: 'flex', alignItems: 'baseline', gap: 1.5, mb: intro ? 0.75 : 2.5}}>
        <Typography
          component="h2"
          id={id}
          sx={{fontSize: '20px', fontWeight: 600, letterSpacing: '-0.02em', m: 0, color: 'text.primary'}}
        >
          {title}
        </Typography>
        {meta && (
          <Typography
            component="span"
            sx={{
              fontFamily: 'monospace',
              fontSize: '10.5px',
              letterSpacing: '0.1em',
              textTransform: 'uppercase',
              color: ink(0.4, 0.3),
            }}
          >
            {meta}
          </Typography>
        )}
      </Box>
      {intro && (
        <Typography sx={{fontSize: '14px', lineHeight: 1.65, color: ink(0.6, 0.5), maxWidth: 660, mb: 2.75}}>
          {intro}
        </Typography>
      )}
    </>
  );
}

/**
 * Copy-to-clipboard button that reports success inline.
 *
 * Clipboard access throws in insecure contexts and when the user has denied it,
 * so a failure leaves the icon unchanged rather than claiming a copy happened.
 */
export function CopyButton({value, accent}: {value: string; accent: string}): JSX.Element {
  const ink = useInk();
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timer.current), []);

  return (
    <Box
      component="button"
      type="button"
      title="Copy"
      aria-label={copied ? 'Copied' : 'Copy to clipboard'}
      onClick={() => {
        navigator.clipboard
          ?.writeText(value)
          .then(() => {
            setCopied(true);
            clearTimeout(timer.current);
            timer.current = setTimeout(() => setCopied(false), 1600);
          })
          .catch(() => {
            // Denied or unavailable; the snippet is still selectable by hand.
          });
      }}
      sx={{
        width: 28,
        height: 28,
        flexShrink: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        borderRadius: '7px',
        border: '1px solid',
        borderColor: ink(0.12, 0.12),
        bgcolor: ink(0.02, 0.04),
        color: copied ? '#4ade80' : ink(0.45, 0.5),
        cursor: 'pointer',
        transition: 'all 0.15s',
        '&:hover': {borderColor: `color-mix(in srgb, ${accent} 50%, transparent)`, color: 'text.primary'},
      }}
    >
      {copied ? <Check size={13} strokeWidth={2.5} /> : <Copy size={13} strokeWidth={2} />}
    </Box>
  );
}

/** Code block with an optional title bar and copy button. */
export function CodeCard({
  code,
  accent,
  copyable = true,
}: {
  code: EcosystemCode;
  accent: string;
  copyable?: boolean;
}): JSX.Element {
  const ink = useInk();
  const hasBar = Boolean(code.file ?? code.lang);

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: ink(0.08, 0.07),
        bgcolor: ink(0.02, 0.12),
        borderRadius: '11px',
        overflow: 'hidden',
      }}
    >
      {hasBar && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 1.25,
            px: 1.75,
            py: 1,
            borderBottom: '1px solid',
            borderColor: ink(0.05, 0.05),
            bgcolor: ink(0.015, 0.02),
          }}
        >
          <Typography component="span" sx={{fontFamily: 'monospace', fontSize: '10.5px', color: ink(0.45, 0.35)}}>
            {code.file}
          </Typography>
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <Typography
              component="span"
              sx={{
                fontFamily: 'monospace',
                fontSize: '9.5px',
                letterSpacing: '0.1em',
                textTransform: 'uppercase',
                color: ink(0.35, 0.25),
              }}
            >
              {code.lang}
            </Typography>
            {copyable && <CopyButton value={code.content} accent={accent} />}
          </Box>
        </Box>
      )}
      <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1.25, px: 2, py: 1.875}}>
        <Box
          component="pre"
          sx={{
            m: 0,
            flex: 1,
            minWidth: 0,
            overflowX: 'auto',
            fontFamily: 'monospace',
            fontSize: '12.5px',
            lineHeight: 1.75,
            color: ink(0.75, 0.72),
          }}
        >
          {code.content}
        </Box>
        {!hasBar && copyable && <CopyButton value={code.content} accent={accent} />}
      </Box>
    </Box>
  );
}

/** Pill-shaped tab strip, used for install surfaces and package managers. */
export function TabStrip({
  labels,
  active,
  onSelect,
  accent,
}: {
  labels: string[];
  active: number;
  onSelect: (index: number) => void;
  accent: string;
}): JSX.Element {
  const ink = useInk();
  return (
    <Box role="tablist" sx={{display: 'flex', flexWrap: 'wrap', gap: 0.75}}>
      {labels.map((label, index) => {
        const isActive = index === active;
        return (
          <Box
            key={label}
            component="button"
            type="button"
            role="tab"
            aria-selected={isActive}
            onClick={() => onSelect(index)}
            sx={{
              px: 1.75,
              py: 0.875,
              borderRadius: '999px',
              fontSize: '12.5px',
              fontWeight: 500,
              fontFamily: 'inherit',
              cursor: 'pointer',
              whiteSpace: 'nowrap',
              transition: 'all 0.15s',
              border: '1px solid',
              borderColor: isActive ? `color-mix(in srgb, ${accent} 45%, transparent)` : ink(0.1, 0.1),
              bgcolor: isActive ? `color-mix(in srgb, ${accent} 13%, transparent)` : 'transparent',
              color: isActive ? 'text.primary' : ink(0.5, 0.5),
            }}
          >
            {label}
          </Box>
        );
      })}
    </Box>
  );
}

/** Left-ruled callout attached to a step or an API entry. */
export function Note({children, accent, label = 'Note'}: {label?: string; children: ReactNode; accent: string}): JSX.Element {
  const ink = useInk();
  return (
    <Box
      sx={{
        display: 'flex',
        gap: 1.25,
        alignItems: 'flex-start',
        mt: 1.5,
        px: 1.75,
        py: 1.375,
        borderLeft: '2px solid',
        borderColor: `color-mix(in srgb, ${accent} 45%, transparent)`,
        bgcolor: `color-mix(in srgb, ${accent} 5%, transparent)`,
        borderRadius: '0 8px 8px 0',
      }}
    >
      <Typography
        component="span"
        sx={{
          fontFamily: 'monospace',
          fontSize: '9px',
          letterSpacing: '0.12em',
          textTransform: 'uppercase',
          color: accent,
          flexShrink: 0,
          pt: '2px',
        }}
      >
        {label}
      </Typography>
      <Typography sx={{fontSize: '12.5px', lineHeight: 1.62, color: ink(0.55, 0.5), m: 0}}>{children}</Typography>
    </Box>
  );
}

/**
 * Call-to-action button.
 *
 * Internal targets go through `useDocsUrl` so a `/docs/next/...` route in the
 * registry follows the reader's version, matching how the listing cards behave.
 */
export function Cta({cta, accent}: {cta: EcosystemCta; accent: string}): JSX.Element {
  const ink = useInk();
  const external = cta.external ?? cta.href.startsWith('http');
  const primary = cta.variant === 'primary';
  const href = external ? cta.href : cta.href;

  const sx = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 0.875,
    px: primary ? 2.5 : 2.25,
    py: 1.25,
    borderRadius: '8px',
    fontSize: '13.5px',
    fontWeight: primary ? 600 : 500,
    textDecoration: 'none',
    transition: 'all 0.2s',
    ...(primary
      ? {
          color: '#fff',
          background: `linear-gradient(135deg, #2560d9 0%, ${accent} 100%)`,
          '&:hover': {transform: 'translateY(-1px)', color: '#fff'},
        }
      : {
          color: ink(0.7, 0.75),
          border: '1px solid',
          borderColor: ink(0.14, 0.14),
          bgcolor: ink(0.02, 0.03),
          '&:hover': {borderColor: `color-mix(in srgb, ${accent} 45%, transparent)`, color: 'text.primary'},
        }),
  };

  if (external) {
    return (
      <Box component="a" href={href} target="_blank" rel="noopener noreferrer" sx={sx}>
        {cta.label}
        <ArrowUpRight size={13} strokeWidth={2.2} />
      </Box>
    );
  }

  return (
    <Box component={Link} to={href} sx={sx}>
      {cta.label}
    </Box>
  );
}
