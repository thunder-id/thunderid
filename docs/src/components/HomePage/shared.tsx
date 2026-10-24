// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import {Box, Typography, type SxProps, type Theme} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import type {ElementType, JSX, ReactNode} from 'react';
import {ACCENTS, type AccentKey, DISPLAY_FONT, EASE, gradientTextSx, sxList, useHomeTokens} from './homeTheme';
import useScrollAnimation from '../../hooks/useScrollAnimation';

/* ─── Reveal on scroll ───────────────────────────────────────────────────── */

interface RevealProps {
  children: ReactNode;
  /** Delay in milliseconds before the element fades in. */
  delay?: number;
  component?: ElementType;
  sx?: SxProps<Theme>;
}

export function Reveal({children, delay = 0, component = 'div', sx = undefined}: RevealProps): JSX.Element {
  const {ref, isVisible} = useScrollAnimation({threshold: 0.12});
  return (
    <Box
      ref={ref}
      component={component}
      sx={[
        {
          opacity: isVisible ? 1 : 0,
          transform: isVisible ? 'none' : 'translateY(26px)',
          transition: `opacity 0.7s ${EASE}, transform 0.7s ${EASE}`,
          transitionDelay: `${delay}ms`,
          '@media (prefers-reduced-motion: reduce)': {opacity: 1, transform: 'none', transition: 'none'},
        },
        ...sxList(sx),
      ]}
    >
      {children}
    </Box>
  );
}

/* ─── Section shell + heading ────────────────────────────────────────────── */

export function Section({id = undefined, children, sx = undefined}: {id?: string; children: ReactNode; sx?: SxProps<Theme>}): JSX.Element {
  return (
    <Box
      component="section"
      id={id}
      sx={[{position: 'relative', py: {xs: 5, md: 7, lg: 9}, scrollMarginTop: 'var(--ifm-navbar-height)'}, ...sxList(sx)]}
    >
      {children}
    </Box>
  );
}

interface SectionHeadingProps {
  eyebrow: string;
  title: ReactNode;
  /** Trailing part of the title, rendered in the muted colour. */
  titleMuted?: ReactNode;
  description?: ReactNode;
  sx?: SxProps<Theme>;
}

export function SectionHeading({
  eyebrow,
  title,
  titleMuted = undefined,
  description = undefined,
  sx = undefined,
}: SectionHeadingProps): JSX.Element {
  return (
    <Box sx={[{maxWidth: 760, mb: {xs: 5, md: 8}}, ...sxList(sx)]}>
      <Reveal>
        <Typography component="div" sx={{fontSize: '0.875rem', fontWeight: 600, mb: 2, ...gradientTextSx}}>
          {eyebrow}
        </Typography>
      </Reveal>
      <Reveal delay={80}>
        <Typography
          variant="h2"
          sx={{
            fontFamily: DISPLAY_FONT,
            fontSize: {xs: '2rem', sm: '2.5rem', md: '3.5rem'},
            fontWeight: 700,
            lineHeight: 1.08,
            letterSpacing: '-0.025em',
            color: 'text.primary',
            textWrap: 'balance',
            m: 0,
          }}
        >
          {title}
          {titleMuted && (
            <>
              {' '}
              <Box component="span" sx={{color: 'text.secondary'}}>
                {titleMuted}
              </Box>
            </>
          )}
        </Typography>
      </Reveal>
      {description && (
        <Reveal delay={160}>
          <Typography
            sx={{mt: 2.25, color: 'text.secondary', fontSize: {xs: '1.0625rem', md: '1.25rem'}, lineHeight: 1.6, maxWidth: '62ch'}}
          >
            {description}
          </Typography>
        </Reveal>
      )}
    </Box>
  );
}

/* ─── Small building blocks ──────────────────────────────────────────────── */

interface TextLinkProps {
  href: string;
  children: ReactNode;
  sx?: SxProps<Theme>;
}

/** Inline "label →" link used under section copy and in cards. */
export function TextLink({href, children, sx = undefined}: TextLinkProps): JSX.Element {
  const t = useHomeTokens();
  const external = /^https?:\/\//.test(href);
  return (
    <Box
      component={Link}
      to={href}
      {...(external ? {target: '_blank', rel: 'noopener noreferrer'} : {})}
      sx={[
        {
          display: 'inline-flex',
          alignItems: 'center',
          gap: 0.75,
          color: t.link,
          fontWeight: 600,
          fontSize: '0.9375rem',
          textDecoration: 'none',
          '& svg': {transition: 'transform 0.2s'},
          '&:hover': {color: t.link, textDecoration: 'none'},
          '&:hover svg': {transform: 'translateX(4px)'},
        },
        ...sxList(sx),
      ]}
    >
      <span>{children}</span>
      <ArrowRight size={16} strokeWidth={2} />
    </Box>
  );
}

/** Rounded, tinted square that frames a line icon in an accent colour. */
export function IconTile({
  accent,
  size = 44,
  children,
  sx = undefined,
}: {
  accent: AccentKey;
  size?: number;
  children: ReactNode;
  sx?: SxProps<Theme>;
}): JSX.Element {
  const c = ACCENTS[accent];
  return (
    <Box
      component="span"
      sx={[
        {
          width: size,
          height: size,
          flex: 'none',
          borderRadius: size > 48 ? '12px' : '10px',
          display: 'grid',
          placeItems: 'center',
          color: c,
          bgcolor: `color-mix(in srgb, ${c} 12%, transparent)`,
          border: `1px solid color-mix(in srgb, ${c} 25%, transparent)`,
        },
        ...sxList(sx),
      ]}
    >
      {children}
    </Box>
  );
}

/**
 * Renders a monochrome SVG logo from `static/assets/images/home/logos` as a CSS mask,
 * so it takes the current text colour in both light and dark mode.
 */
export function MaskLogo({
  name,
  width,
  height,
  label = undefined,
  sx = undefined,
}: {
  name: string;
  width: number | string;
  height: number | string;
  label?: string;
  sx?: SxProps<Theme>;
}): JSX.Element {
  const url = useBaseUrl(`/assets/images/home/logos/${name}.svg`);
  return (
    <Box
      component="span"
      role={label ? 'img' : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      sx={[
        {
          display: 'inline-block',
          flex: 'none',
          width,
          height,
          bgcolor: 'currentColor',
          mask: `url("${url}") center / contain no-repeat`,
          WebkitMask: `url("${url}") center / contain no-repeat`,
        },
        ...sxList(sx),
      ]}
    />
  );
}
