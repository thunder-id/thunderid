// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, ButtonBase, Container, Typography} from '@wso2/oxygen-ui';
import {type JSX, type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState} from 'react';
import {BRAND_GRADIENT, DISPLAY_FONT, EASE, useHomeTokens, HOME_MAX_WIDTH} from './homeTheme';
import {Reveal, Section, SectionHeading, TextLink} from './shared';
import useScrollAnimation from '../../hooks/useScrollAnimation';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

/* ─── Code samples ───────────────────────────────────────────────────────────
 * Lines use `{x|text}` markers for syntax colours:
 *   k keyword · s string · f function · p property · c comment · n number
 */

interface TokenStyle {
  color: string;
  fontStyle?: 'italic';
}

/** Dark mode follows the design; light mode matches the site's Night Owl Light code blocks. */
const TOKEN_STYLES: Record<'dark' | 'light', Record<string, TokenStyle>> = {
  dark: {
    k: {color: '#ff7b72'},
    s: {color: '#a5d6ff'},
    f: {color: '#d2a8ff'},
    p: {color: '#79c0ff'},
    c: {color: '#8b949e'},
    n: {color: '#79c0ff'},
  },
  light: {
    k: {color: 'rgb(12, 150, 155)'},
    s: {color: 'rgb(72, 118, 214)'},
    f: {color: 'rgb(153, 76, 195)', fontStyle: 'italic'},
    p: {color: 'rgb(72, 118, 214)', fontStyle: 'italic'},
    c: {color: 'rgb(152, 159, 177)', fontStyle: 'italic'},
    n: {color: 'rgb(170, 9, 130)'},
  },
};

interface Panel {
  tab: string;
  file: string;
  title: string;
  text: string;
  link: {label: string; href: string};
  code: string[];
}

const PANELS: Panel[] = [
  {
    tab: 'Applications',
    file: 'app.js',
    title: 'Add sign-in to any app',
    text: 'Drop in the official SDK for React, Next.js, Vue, Nuxt, Express, Node.js, iOS, Android, or Flutter and ship secure auth today.',
    link: {label: 'Browse quickstarts', href: '/sdks-and-tools'},
    code: [
      "{k|import} { ThunderIDClient } {k|from} {s|'@thunderid/browser'};",
      '',
      '{k|const} auth = {k|new} {f|ThunderIDClient}({',
      "  {p|baseUrl}: {s|'https://localhost:8090'},",
      "  {p|clientId}: {s|'<your-client-id>'},",
      '  {p|redirectUrl}: window.location.origin,',
      '});',
      '',
      "document.{f|querySelector}({s|'#login'}).onclick = () => auth.{f|signIn}();",
    ],
  },
  {
    tab: 'AI agents',
    file: 'agent-token.sh',
    title: 'Give agents their own identity',
    text: 'Issue scoped, short-lived tokens to agents and machines with standard OAuth grants, with no shared secrets in prompts.',
    link: {label: 'Secure your first agent', href: '/docs/next/getting-started/connect-your-agent'},
    code: [
      '{c|# Standard OAuth 2.0 client-credentials grant}',
      '{f|curl} -X POST https://localhost:8090/oauth2/token \\',
      '  -u {s|"$AGENT_ID:$AGENT_SECRET"} \\',
      '  -d {s|"grant_type=client_credentials"} \\',
      '  -d {s|"scope=orders:read"}',
      '',
      '{c|# { "access_token": "eyJ…", "expires_in": 3600 }}',
    ],
  },
  {
    tab: 'Flows',
    file: 'login-flow.yaml',
    title: 'Compose identity flows',
    text: 'Model every step, from password and social login to MFA and recovery, as a declarative flow you can review in a pull request.',
    link: {label: 'Explore flows', href: '/docs/next/guides/flows/what-are-flows'},
    code: [
      '{p|flow}: login',
      '{p|steps}:',
      '  - {p|id}: identify',
      '    {p|type}: username_password',
      '  - {p|id}: mfa',
      '    {p|type}: totp',
      '    {p|when}: risk.score > {n|0.6}',
      '  - {p|id}: done',
      '    {p|type}: issue_tokens',
    ],
  },
  {
    tab: 'Deploy',
    file: 'terminal',
    title: 'Run it anywhere',
    text: 'Start ThunderID with one command, then ship the same config to Docker, Kubernetes, or Helm with a GitOps workflow.',
    link: {label: 'Get ThunderID', href: '/docs/next/getting-started/get-thunderid'},
    code: [
      '{c|# run ThunderID locally}',
      '{f|npx} thunderid',
      '',
      '{c|# or deploy with Docker, Kubernetes, or Helm}',
      '{c|# see Get ThunderID for all platforms}',
    ],
  },
];

const STATS = [
  {value: 10, prefix: '', suffix: '+', label: 'Official SDKs, from React to Flutter'},
  {
    value: 6,
    prefix: '',
    suffix: '+',
    label: 'Identity types: humans, AI agents, machines, applications, wallets, MCP and more',
  },
  {value: 100, prefix: '', suffix: '%', label: 'Open source, Apache-2.0'},
  {value: 50, prefix: '<', suffix: 'ms', label: 'Startup time for the Go runtime'},
];

const ROTATE_MS = 7000;

function renderLine(line: string, styles: Record<string, TokenStyle>): JSX.Element[] {
  const out: JSX.Element[] = [];
  const re = /\{([kspfcn])\|((?:[^{}]|\{[^{}]*\})*)\}/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(line))) {
    if (m.index > last) out.push(<span key={i++}>{line.slice(last, m.index)}</span>);
    out.push(
      <span key={i++} style={styles[m[1]]}>
        {m[2]}
      </span>,
    );
    last = re.lastIndex;
  }
  if (last < line.length) out.push(<span key={i++}>{line.slice(last)}</span>);
  return out;
}

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/* ─── Stat with count-up ─────────────────────────────────────────────────── */

function Stat({stat, delay}: {stat: (typeof STATS)[number]; delay: number}): JSX.Element {
  const t = useHomeTokens();
  const {ref, isVisible} = useScrollAnimation({threshold: 0.3});
  const [shown, setShown] = useState(stat.value);

  useEffect(() => {
    if (!isVisible || prefersReducedMotion()) return undefined;
    let raf = 0;
    const start = performance.now() + delay;
    const step = (now: number): void => {
      const p = Math.min(1, Math.max(0, (now - start) / 1400));
      setShown(Math.round(stat.value * (1 - Math.pow(1 - p, 3))));
      if (p < 1) raf = requestAnimationFrame(step);
    };
    // The first frame lands before `start`, so the counter resets to 0 there.
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [isVisible, stat.value, delay]);

  return (
    <Box
      ref={ref}
      sx={{
        p: '26px 24px',
        borderRadius: '12px',
        bgcolor: t.surface,
        border: '1px solid',
        borderColor: t.line,
        borderTop: '2px solid',
        borderTopColor: 'primary.main',
        backdropFilter: 'blur(12px)',
        opacity: isVisible ? 1 : 0,
        transform: isVisible ? 'none' : 'translateY(26px)',
        transition: `opacity 0.7s ${EASE} ${delay}ms, transform 0.7s ${EASE} ${delay}ms`,
      }}
    >
      <Box
        sx={{
          display: 'inline-block',
          fontFamily: DISPLAY_FONT,
          fontSize: {xs: '2.125rem', md: '3.25rem'},
          fontWeight: 700,
          letterSpacing: '-0.03em',
          lineHeight: 1.05,
          fontVariantNumeric: 'tabular-nums',
          background: BRAND_GRADIENT,
          WebkitBackgroundClip: 'text',
          backgroundClip: 'text',
          color: 'transparent',
        }}
      >
        {stat.prefix}
        {shown}
        {stat.suffix}
      </Box>
      <Typography sx={{mt: 0.75, fontSize: '0.9rem', lineHeight: 1.5, color: 'text.secondary'}}>
        {stat.label}
      </Typography>
    </Box>
  );
}

/* ─── Section ────────────────────────────────────────────────────────────── */

export default function DevelopersSection(): JSX.Element {
  const t = useHomeTokens();
  const docsUrl = useDocsUrl();
  const [active, setActive] = useState(0);
  const [userPicked, setUserPicked] = useState(false);
  const [hovering, setHovering] = useState(false);
  const [indicator, setIndicator] = useState({left: 0, width: 0});
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  // Slide the pill indicator under the active tab.
  useLayoutEffect(() => {
    const place = (): void => {
      const el = tabRefs.current[active];
      if (el) setIndicator({left: el.offsetLeft, width: el.offsetWidth});
    };
    place();
    window.addEventListener('resize', place);
    void document.fonts?.ready.then(place);
    return () => window.removeEventListener('resize', place);
  }, [active]);

  // Rotate through the samples until the reader interacts.
  useEffect(() => {
    if (userPicked || hovering || prefersReducedMotion()) return undefined;
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') setActive((a) => (a + 1) % PANELS.length);
    }, ROTATE_MS);
    return () => window.clearInterval(id);
  }, [userPicked, hovering]);

  const pick = (i: number, focus = false): void => {
    setUserPicked(true);
    setActive(i);
    if (focus) tabRefs.current[i]?.focus();
  };

  const onKeyDown = (e: KeyboardEvent): void => {
    if (e.key !== 'ArrowRight' && e.key !== 'ArrowLeft') return;
    e.preventDefault();
    pick((active + (e.key === 'ArrowRight' ? 1 : -1) + PANELS.length) % PANELS.length, true);
  };

  const panel = PANELS[active];
  const tokenStyles = TOKEN_STYLES[t.isDark ? 'dark' : 'light'];
  const code = t.isDark
    ? {
        bg: 'rgba(13,17,23,0.92)',
        bar: 'transparent',
        text: '#f0f8ff',
        dim: 'rgba(255,255,255,0.7)',
        tab: 'rgba(200,225,255,0.05)',
        lineNo: '#5b6b85',
        divider: 'rgba(160,200,255,0.10)',
      }
    : {
        bg: '#FBFBFB',
        bar: 'rgba(54,136,255,0.06)',
        text: '#403f53',
        dim: 'rgba(64,63,83,0.75)',
        tab: 'rgba(54,136,255,0.08)',
        lineNo: 'rgba(64,63,83,0.4)',
        divider: 'rgba(15,40,80,0.10)',
      };

  return (
    <Section id="developers">
      <Container maxWidth={false} sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}}}>
        <SectionHeading
          eyebrow="Developers"
          title={
            <>
              From{' '}
              <Box
                component="code"
                sx={{
                  fontWeight: 500,
                  color: t.link,
                  fontSize: '0.86em',
                  letterSpacing: '-0.04em',
                  bgcolor: 'transparent',
                  border: 0,
                  p: 0,
                }}
              >
                npx thunderid
              </Box>{' '}
              to secured agents
            </>
          }
          titleMuted="in a few lines."
          description="SDKs for the frameworks you already use, a CLI for everything else, and APIs your agents can call safely."
        />

        <Reveal delay={200}>
          <Box
            role="tablist"
            aria-label="Use cases"
            sx={{
              position: 'relative',
              display: 'inline-flex',
              maxWidth: '100%',
              overflowX: 'auto',
              scrollbarWidth: 'none',
              p: 0.5,
              borderRadius: '999px',
              bgcolor: t.isDark ? 'rgba(13,17,23,0.7)' : 'rgba(255,255,255,0.8)',
              border: '1px solid',
              borderColor: t.isDark ? '#1f3350' : t.lineStrong,
              backdropFilter: 'blur(12px)',
              '&::-webkit-scrollbar': {display: 'none'},
            }}
          >
            <Box
              aria-hidden
              sx={{
                position: 'absolute',
                top: 4,
                bottom: 4,
                left: 0,
                width: indicator.width,
                transform: `translateX(${indicator.left}px)`,
                borderRadius: '999px',
                bgcolor: t.isDark ? 'rgba(200,225,255,0.1)' : 'rgba(54,136,255,0.1)',
                border: '1px solid',
                borderColor: t.lineStrong,
                transition: `transform 0.35s ${EASE}, width 0.35s ${EASE}`,
              }}
            />
            {PANELS.map((p, i) => (
              <ButtonBase
                key={p.tab}
                ref={(el: HTMLButtonElement | null) => {
                  tabRefs.current[i] = el;
                }}
                role="tab"
                aria-selected={i === active}
                aria-controls="developers-panel"
                tabIndex={i === active ? 0 : -1}
                onClick={() => pick(i)}
                onKeyDown={onKeyDown}
                sx={{
                  position: 'relative',
                  zIndex: 1,
                  px: 2.25,
                  py: 1,
                  borderRadius: '999px',
                  fontSize: '0.9rem',
                  fontWeight: 500,
                  whiteSpace: 'nowrap',
                  color: i === active ? 'text.primary' : 'text.secondary',
                  transition: 'color 0.25s',
                }}
              >
                {p.tab}
              </ButtonBase>
            ))}
          </Box>
        </Reveal>

        <Box
          sx={{
            mt: 3.5,
            display: 'grid',
            gridTemplateColumns: {xs: 'minmax(0, 1fr)', md: 'minmax(0, 1fr) minmax(0, 1.25fr)'},
            gap: {xs: 3.5, md: 7},
            alignItems: 'center',
          }}
        >
          <Reveal delay={120} sx={{minHeight: {md: 220}}}>
            <Box
              key={active}
              sx={{
                '@keyframes devCopyIn': {
                  from: {opacity: 0, transform: 'translateY(12px)'},
                  to: {opacity: 1, transform: 'none'},
                },
                animation: 'devCopyIn 0.35s ease both',
              }}
            >
              <Typography
                component="h3"
                sx={{m: 0, fontFamily: DISPLAY_FONT, fontSize: '1.5rem', fontWeight: 600, letterSpacing: '-0.015em'}}
              >
                {panel.title}
              </Typography>
              <Typography sx={{mt: 1.5, fontSize: '1rem', lineHeight: 1.65, color: 'text.secondary', maxWidth: '44ch'}}>
                {panel.text}
              </Typography>
              <TextLink href={docsUrl(panel.link.href)} sx={{mt: 2.5}}>
                {panel.link.label}
              </TextLink>
            </Box>
          </Reveal>

          <Reveal delay={160} sx={{minWidth: 0}}>
            <Box
              id="developers-panel"
              role="tabpanel"
              onMouseEnter={() => setHovering(true)}
              onMouseLeave={() => setHovering(false)}
              sx={{
                position: 'relative',
                overflow: 'hidden',
                borderRadius: '20px',
                bgcolor: code.bg,
                border: '1px solid',
                borderColor: t.isDark ? 'rgba(160,200,255,0.18)' : 'rgba(15,40,80,0.12)',
                boxShadow: t.isDark ? t.shadow : '0 18px 40px -28px rgba(15,40,80,0.25)',
                backdropFilter: 'blur(18px)',
                color: code.text,
              }}
            >
              <Box
                sx={{
                  height: 42,
                  px: 1.75,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  bgcolor: code.bar,
                  borderBottom: `1px solid ${code.divider}`,
                }}
              >
                {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
                  <Box key={c} component="span" sx={{width: 11, height: 11, borderRadius: '50%', bgcolor: c}} />
                ))}
                <Box
                  component="span"
                  sx={{
                    ml: 1.25,
                    alignSelf: 'flex-end',
                    px: 1.5,
                    py: 0.75,
                    borderRadius: '6px 6px 0 0',
                    fontFamily: 'var(--ifm-font-family-monospace)',
                    fontSize: '0.78rem',
                    color: code.dim,
                    bgcolor: code.tab,
                  }}
                >
                  {panel.file}
                </Box>
                <Box
                  component="span"
                  sx={{
                    ml: 'auto',
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: 0.875,
                    fontSize: '0.78rem',
                    fontWeight: 600,
                    color: '#3fb950',
                  }}
                >
                  <Box component="i" sx={{width: 7, height: 7, borderRadius: '50%', bgcolor: '#3fb950'}} />
                  Live
                </Box>
              </Box>
              <Box
                component="pre"
                key={active}
                sx={{
                  m: 0,
                  py: 2.5,
                  px: 0,
                  minHeight: 300,
                  overflowX: 'auto',
                  bgcolor: 'transparent',
                  border: 0,
                  borderRadius: 0,
                  fontFamily: 'var(--ifm-font-family-monospace)',
                  fontSize: '0.84rem',
                  lineHeight: 1.75,
                  color: code.text,
                  '@keyframes devLineIn': {
                    from: {opacity: 0, transform: 'translateX(6px)'},
                    to: {opacity: 1, transform: 'none'},
                  },
                }}
              >
                {panel.code.map((line, n) => (
                  <Box
                    // eslint-disable-next-line react/no-array-index-key -- lines are static and positional
                    key={n}
                    component="span"
                    sx={{
                      display: 'block',
                      pr: 2.5,
                      whiteSpace: 'pre',
                      animation: `devLineIn 0.3s ease ${60 + n * 45}ms both`,
                      '@media (prefers-reduced-motion: reduce)': {animation: 'none'},
                    }}
                  >
                    <Box
                      component="span"
                      sx={{
                        display: 'inline-block',
                        width: 44,
                        pr: 2,
                        textAlign: 'right',
                        color: code.lineNo,
                        userSelect: 'none',
                      }}
                    >
                      {n + 1}
                    </Box>
                    {line ? renderLine(line, tokenStyles) : ' '}
                  </Box>
                ))}
              </Box>
            </Box>
          </Reveal>
        </Box>

        <Box
          sx={{
            mt: {xs: 7, md: 12},
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(4, 1fr)'},
            gap: 2,
          }}
        >
          {STATS.map((s, i) => (
            <Stat key={s.label} stat={s} delay={i * 80} />
          ))}
        </Box>
      </Container>
    </Section>
  );
}
