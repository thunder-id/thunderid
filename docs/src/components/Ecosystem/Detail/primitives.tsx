// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {Highlight, themes} from 'prism-react-renderer';
import {JSX, KeyboardEvent, MutableRefObject, ReactNode, useEffect, useId, useRef, useState} from 'react';
import {useInk} from './theme';
import useIsDarkMode from '@site/src/hooks/useIsDarkMode';
import type {EcosystemCode, EcosystemCta} from '@site/src/types/ecosystem';

// Same Prism themes docusaurus.config.ts configures for the docs site's own `@theme/CodeBlock`
// (`prism.theme` / `prism.darkTheme`), so a code block here and one in an .mdx doc are
// highlighted identically rather than looking like two different systems.
const CODE_THEME = {light: themes.nightOwlLight, dark: themes.nightOwl};

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

/** One tab in a `CodeCard`'s package-manager (or similar) switcher. */
export interface CodeCardTab {
  value: string;
  label: string;
  icon?: string;
  active: boolean;
  onSelect: () => void;
}

/**
 * Tab switcher rendered inside a `CodeCard`'s bar, in the slot `code.file` would
 * otherwise take — same greyscale-except-active-icon treatment as the .mdx docs'
 * package-manager tabs (CodeGroup.tsx / `.tid-codeblock__tab` in custom.css), so the
 * two systems' tab strips read as the same component.
 */
function CodeCardTabStrip({
  tabs,
  baseId,
  panelId,
  tabRefs,
  onKeyDown,
}: {
  tabs: CodeCardTab[];
  baseId: string;
  panelId: string;
  tabRefs: MutableRefObject<(HTMLButtonElement | null)[]>;
  onKeyDown: (event: KeyboardEvent<HTMLElement>) => void;
}): JSX.Element {
  const ink = useInk();
  return (
    <Box
      sx={{display: 'flex', flex: '1 1 auto', flexWrap: 'wrap', alignItems: 'center', rowGap: 0.5, columnGap: 1.25, minWidth: 0}}
      role="tablist"
      tabIndex={-1}
      onKeyDown={onKeyDown}
    >
      {tabs.map((tab, index) => (
        <Box
          key={tab.value}
          ref={(el: HTMLButtonElement | null) => {
            tabRefs.current[index] = el;
          }}
          id={`${baseId}-tab-${tab.value}`}
          component="button"
          type="button"
          role="tab"
          aria-selected={tab.active}
          aria-controls={panelId}
          tabIndex={tab.active ? 0 : -1}
          onClick={tab.onSelect}
          sx={{
            display: 'flex',
            flexShrink: 0,
            alignItems: 'center',
            gap: 0.5,
            p: 0,
            border: 'none',
            bgcolor: 'transparent',
            font: 'inherit',
            fontFamily: 'monospace',
            fontSize: '10.5px',
            fontWeight: 600,
            color: tab.active ? 'text.primary' : ink(0.4, 0.35),
            cursor: 'pointer',
            whiteSpace: 'nowrap',
          }}
        >
          {tab.icon && (
            <Box
              component="img"
              src={tab.icon}
              alt=""
              sx={{
                height: '1em',
                width: '1em',
                filter: tab.active ? 'none' : 'grayscale(1) opacity(0.55)',
                transition: 'filter 0.15s ease',
              }}
            />
          )}
          {tab.label}
        </Box>
      ))}
    </Box>
  );
}

/** Code block with an optional title bar and copy button. */
export function CodeCard({
  code,
  accent,
  copyable = true,
  tabs = undefined,
}: {
  code: EcosystemCode;
  accent: string;
  copyable?: boolean;
  /** Package-manager switcher rendered in place of `code.file` on the bar's left. */
  tabs?: CodeCardTab[];
}): JSX.Element {
  const ink = useInk();
  const isLight = !useIsDarkMode();
  const hasBar = Boolean(code.file ?? code.lang) || Boolean(tabs?.length);
  // The theme's own background/foreground, not the `ink()` navy tint used elsewhere on this
  // page — this is what makes a code block here render the exact same colour as one in an
  // .mdx doc (both draw from the same nightOwlLight/nightOwl theme), rather than looking like
  // a differently-tinted card that merely uses the same token colours.
  const {plain} = isLight ? CODE_THEME.light : CODE_THEME.dark;
  const baseId = useId();
  const panelId = `${baseId}-panel`;
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  // Arrow keys both move focus and activate the tab (WAI-ARIA's "automatic activation" tabs
  // pattern), matching the .mdx docs' package-manager tabs (CodeBlock/Layout).
  const handleTabsKeyDown = (event: KeyboardEvent<HTMLElement>): void => {
    if (!tabs || (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight')) return;
    event.preventDefault();
    const current = tabs.findIndex((tab) => tab.active);
    const delta = event.key === 'ArrowRight' ? 1 : -1;
    const nextIndex = (current + delta + tabs.length) % tabs.length;
    tabs[nextIndex]?.onSelect();
    tabRefs.current[nextIndex]?.focus();
  };

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: ink(0.08, 0.07),
        bgcolor: plain.backgroundColor,
        borderRadius: '11px',
        overflow: 'hidden',
      }}
    >
      {hasBar && (
        <Box
          sx={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'space-between',
            rowGap: 0.5,
            columnGap: 1.25,
            px: 1.75,
            py: 1,
            borderBottom: '1px solid',
            // Same `color-mix(accent, ink)` formula as the .mdx bar's CSS (`.tid-codeblock__bar`
            // in custom.css, mixing `var(--ifm-color-primary)` there) — a flat ink()-only overlay
            // read as barely-there next to the real nightOwl background above, where the .mdx
            // bar's colour tint gave it visible weight against the same background.
            borderColor: `color-mix(in srgb, ${accent} ${isLight ? 15 : 20}%, ${ink(0.05, 0.06)})`,
            bgcolor: `color-mix(in srgb, ${accent} ${isLight ? 5 : 8}%, ${ink(0.02, 0.04)})`,
          }}
        >
          {tabs?.length ? (
            <CodeCardTabStrip
              tabs={tabs}
              baseId={baseId}
              panelId={panelId}
              tabRefs={tabRefs}
              onKeyDown={handleTabsKeyDown}
            />
          ) : (
            <Typography component="span" sx={{fontFamily: 'monospace', fontSize: '10.5px', color: ink(0.45, 0.35)}}>
              {code.file}
            </Typography>
          )}
          <Box sx={{display: 'flex', flexShrink: 0, alignItems: 'center', gap: 1}}>
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
      <Box
        sx={{display: 'flex', alignItems: 'flex-start', gap: 1.25, px: 2, py: 1.875}}
        {...(tabs?.length
          ? {
              role: 'tabpanel',
              id: panelId,
              'aria-labelledby': `${baseId}-tab-${tabs.find((tab) => tab.active)?.value}`,
              // The highlighted code itself isn't focusable, so the panel needs to be, per the
              // WAI-ARIA tabs pattern, for keyboard users to reach its content after Tab-ing to
              // the active tab.
              tabIndex: 0,
            }
          : {})}
      >
        <Highlight
          theme={isLight ? CODE_THEME.light : CODE_THEME.dark}
          code={code.content}
          language={code.lang ?? 'text'}
        >
          {({className, tokens, getLineProps, getTokenProps}) => (
            <Box
              component="pre"
              className={className}
              sx={{
                m: 0,
                flex: 1,
                minWidth: 0,
                overflowX: 'auto',
                background: 'none',
                fontFamily: 'monospace',
                fontSize: '12.5px',
                lineHeight: 1.75,
              }}
            >
              {tokens.map((line, i) => (
                // Lines/tokens have no stable identity of their own and this is
                // prism-react-renderer's own documented usage pattern.
                // eslint-disable-next-line react/no-array-index-key
                <div key={i} {...getLineProps({line})}>
                  {line.map((token, key) => (
                    // eslint-disable-next-line react/no-array-index-key
                    <span key={key} {...getTokenProps({token})} />
                  ))}
                </div>
              ))}
            </Box>
          )}
        </Highlight>
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
