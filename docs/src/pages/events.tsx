// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Head from '@docusaurus/Head';
import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import {Box, Button, Typography} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Calendar,
  ChevronDown,
  ExternalLink,
  Globe,
  MapPin,
  Mic,
  Sparkles,
  Users,
  type LucideIcon,
} from '@wso2/oxygen-ui-icons-react';
import {useId, useState, type JSX, type MouseEvent, type ReactNode} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {
  EVENTS,
  formatDayRange,
  formatFullDate,
  formatMonth,
  formatPlace,
  formatWeekdays,
  groupBy,
  isExternalUrl,
  isPast,
  TYPE_RANK,
  useToday,
  type EventType,
  type HubEvent,
} from '@site/src/components/Events/events';
import {
  accentCardSx,
  BADGE_GRADIENT,
  DOT_GRID,
  EVENT_TYPE_TONES,
  linkColor,
  MONO_LABEL_SX,
  outlinedButtonSx,
  primaryButtonSx,
  REDUCED_MOTION,
  SPLIT_PANE_BG,
  surfaceCardSx,
  VISUALLY_HIDDEN_SX,
} from '@site/src/components/Events/eventsTheme';
import {SectionTitle} from '@site/src/components/Events/shared';
import {ACCENTS, DISPLAY_FONT, gradientTextSx, useHomeTokens} from '@site/src/components/HomePage/homeTheme';

// TODO: Replace with the final event intake form URL.
const EVENT_PROPOSAL_URL = 'https://github.com/thunder-id/thunderid/discussions';

/** Query parameter that prefills the request type on the intake form. Adjust it to match the final form. */
const EVENT_PROPOSAL_TYPE_PARAM = 'type';

const proposalUrl = (requestType: string): string => {
  const url = new URL(EVENT_PROPOSAL_URL);
  url.searchParams.set(EVENT_PROPOSAL_TYPE_PARAM, requestType);
  return url.toString();
};

const CANONICAL_URL = 'https://thunderid.dev/events/';

const UPCOMING_ID = 'upcoming';
const PROPOSE_ID = 'propose';

const TYPE_META: Record<EventType, {label: string; icon: LucideIcon}> = {
  hosted: {label: 'Hosted by {{ProductName}}', icon: Sparkles},
  presence: {label: 'Meet the team', icon: Users},
  talk: {label: '{{ProductName}} talk', icon: Mic},
};

const TABULAR_SX = {fontVariantNumeric: 'tabular-nums'} as const;

const GROUP_HEADING_SX = {
  fontFamily: DISPLAY_FONT,
  fontSize: '1.25rem',
  fontWeight: 700,
  letterSpacing: '-0.015em',
  color: 'text.primary',
  m: 0,
  mb: 2,
  ...TABULAR_SX,
} as const;

const INTRO_SX = {color: 'text.secondary', fontSize: '1rem', lineHeight: 1.6, mt: 1.25, mb: {xs: 4, md: 5}} as const;

function useProductName(): string {
  const {siteConfig} = useDocusaurusContext();
  return (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;
}

/** In-page jump that scrolls smoothly (unless reduced motion is set) and moves focus to the target section. */
function jumpTo(id: string) {
  return (e: MouseEvent<HTMLAnchorElement>): void => {
    const target = document.getElementById(id);
    if (!target) return;
    e.preventDefault();
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    target.scrollIntoView({behavior: reduce ? 'auto' : 'smooth', block: 'start'});
    target.focus({preventScroll: true});
    window.history.replaceState(null, '', `#${id}`);
  };
}

/* ─── Small building blocks ──────────────────────────────────────────────── */

function TypeBadge({type}: {type: EventType}): JSX.Element {
  const t = useHomeTokens();
  const productName = useProductName();
  const tone = EVENT_TYPE_TONES[t.isDark ? 'dark' : 'light'][type];
  const {label, icon: Icon} = TYPE_META[type];
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.75,
        px: 1.25,
        py: 0.375,
        borderRadius: '999px',
        border: '1px solid',
        borderColor: tone.border,
        bgcolor: tone.bg,
        color: tone.fg,
        fontSize: '0.8125rem',
        fontWeight: 600,
        lineHeight: 1.4,
        whiteSpace: 'nowrap',
      }}
    >
      <Icon size={14} strokeWidth={2.25} aria-hidden />
      {label.replaceAll('{{ProductName}}', productName)}
    </Box>
  );
}

function Place({event}: {event: HubEvent}): JSX.Element {
  const Icon = event.online ? Globe : MapPin;
  return (
    <Box
      component="span"
      sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75, color: 'text.secondary', fontSize: '0.875rem'}}
    >
      <Icon size={15} strokeWidth={2} aria-hidden />
      {formatPlace(event)}
    </Box>
  );
}

/** "View event", only for events with their own page on this site. Other events carry no link. */
function ViewEventLink({event}: {event: HubEvent}): JSX.Element | null {
  const t = useHomeTokens();
  if (isExternalUrl(event.url)) return null;
  return (
    <Box
      component={Link}
      to={event.url}
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.75,
        minHeight: 44,
        mt: 0.75,
        color: linkColor(t),
        fontWeight: 600,
        fontSize: '0.9375rem',
        textDecoration: 'none',
        '& svg': {transition: 'transform 0.2s'},
        '&:hover': {color: linkColor(t), textDecoration: 'underline', textUnderlineOffset: '3px'},
        '&:hover svg': {transform: 'translateX(4px)'},
        [REDUCED_MOTION]: {'& svg': {transition: 'none'}, '&:hover svg': {transform: 'none'}},
      }}
    >
      View event
      <ArrowRight size={16} strokeWidth={2} aria-hidden />
    </Box>
  );
}

function Talks({event}: {event: HubEvent}): JSX.Element | null {
  if (!event.talks?.length) return null;
  return (
    <Box component="ul" sx={{listStyle: 'none', p: 0, m: 0, mt: 1.5, display: 'grid', gap: 1}}>
      {event.talks.map((talk) => (
        <Box component="li" key={talk.title} sx={{display: 'flex', gap: 1, fontSize: '0.9375rem', lineHeight: 1.5}}>
          <Box component="span" sx={{color: 'primary.main', flex: 'none', pt: '3px'}}>
            <Mic size={15} strokeWidth={2} aria-hidden />
          </Box>
          <span>
            <Box component="span" sx={{color: 'text.primary', fontWeight: 600}}>
              {talk.title}
            </Box>
            {talk.speakers.length > 0 && (
              <Box component="span" sx={{color: 'text.secondary'}}>
                {' '}
                by {talk.speakers.join(', ')}
              </Box>
            )}
          </span>
        </Box>
      ))}
    </Box>
  );
}

/** Hosted events get a link on the title itself, so the anchor text describes the page it points to. */
function EventTitle({event}: {event: HubEvent}): JSX.Element {
  const linked = event.type === 'hosted' && !isExternalUrl(event.url);
  return (
    <Typography
      component="h4"
      sx={{
        fontFamily: DISPLAY_FONT,
        fontSize: event.type === 'talk' ? '1.0625rem' : '1.1875rem',
        fontWeight: 700,
        letterSpacing: '-0.015em',
        lineHeight: 1.3,
        color: 'text.primary',
        m: 0,
        '& a': {color: 'inherit', textDecoration: 'none'},
        '& a:hover': {color: 'primary.main', textDecoration: 'none'},
      }}
    >
      {linked ? <Link to={event.url}>{event.title}</Link> : event.title}
    </Typography>
  );
}

/* ─── Event card, shared by the timeline and past hosted events ─────────── */

function EventCard({event, muted = false}: {event: HubEvent; muted?: boolean}): JSX.Element {
  const t = useHomeTokens();
  const blue = ACCENTS.electric;
  const variant = {
    hosted: {
      ...surfaceCardSx(t),
      p: {xs: 2.5, md: 3},
      background: `linear-gradient(150deg, color-mix(in srgb, ${blue} ${t.isDark ? 16 : 9}%, transparent), transparent 65%), ${t.surfaceStrong}`,
      borderColor: `color-mix(in srgb, ${blue} ${t.isDark ? 45 : 35}%, transparent)`,
      boxShadow: muted ? 'none' : `0 16px 40px -28px color-mix(in srgb, ${blue} 70%, transparent)`,
    },
    presence: {...surfaceCardSx(t), p: {xs: 2.5, md: 3}},
    talk: {...surfaceCardSx(t), p: {xs: 2, md: 2.25}, bgcolor: t.surface, boxShadow: 'none'},
  }[event.type];

  return (
    <Box
      component="article"
      sx={{
        ...variant,
        position: 'relative',
        transition: 'border-color 0.25s ease, transform 0.25s ease',
        '&:hover': {
          borderColor: `color-mix(in srgb, ${blue} 55%, transparent)`,
          transform: 'translateY(-2px)',
        },
        [REDUCED_MOTION]: {transition: 'none', '&:hover': {transform: 'none'}},
      }}
    >
      <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', columnGap: 2, rowGap: 1, mb: 1.25}}>
        <TypeBadge type={event.type} />
        <Place event={event} />
      </Box>
      <EventTitle event={event} />
      <Typography sx={{color: 'text.secondary', fontSize: '0.9375rem', lineHeight: 1.6, mt: 0.75, maxWidth: '68ch'}}>
        {event.summary}
      </Typography>
      <Talks event={event} />
      {/* Reserved for event posters in a later iteration; nothing renders while `poster` is unset. */}
      <ViewEventLink event={event} />
    </Box>
  );
}

/* ─── Left panel: get involved ─────────────────────────────────────────── */

const PROPOSALS: {type: string; icon: LucideIcon; title: string; text: string; cta: string}[] = [
  {
    type: 'meetup',
    icon: Users,
    title: 'Host a {{ProductName}} meetup',
    text: "Run a local meetup and we'll help with speakers, content, and promotion.",
    cta: 'Propose a meetup',
  },
  {
    type: 'present',
    icon: Mic,
    title: 'Invite us to present',
    text: 'Invite a {{ProductName}} maintainer to talk or demo, or partner with us on a booth or workshop.',
    cta: 'Invite us',
  },
];

/** Two compact cards, each one link to the intake form. */
function GetInvolved(): JSX.Element {
  const t = useHomeTokens();
  const productName = useProductName();
  const name = (s: string): string => s.replaceAll('{{ProductName}}', productName);
  return (
    <Box
      component="section"
      id={PROPOSE_ID}
      tabIndex={-1}
      aria-labelledby="get-involved"
      sx={{
        flex: 'none',
        px: {xs: 3, sm: 5, lg: 7},
        pt: 2.75,
        pb: {xs: 5, md: 4},
        borderTop: '1px solid',
        borderColor: t.line,
        scrollMarginTop: 'var(--ifm-navbar-height)',
        '&:focus': {outline: 'none'},
      }}
    >
      <Typography
        id="get-involved"
        component="h2"
        sx={{
          fontFamily: DISPLAY_FONT,
          fontSize: '1.125rem',
          fontWeight: 700,
          letterSpacing: '-0.015em',
          color: 'text.primary',
          m: 0,
          mb: 1.5,
        }}
      >
        Bring {productName} to your community
      </Typography>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr', md: '1fr', lg: '1fr 1fr'}, gap: 1.5}}>
        {PROPOSALS.map(({type, icon: Icon, title, text, cta}) => (
          <Box
            key={type}
            component="a"
            href={proposalUrl(type)}
            target="_blank"
            rel="noopener noreferrer"
            sx={{
              ...accentCardSx(t),
              '&:hover': {...accentCardSx(t)['&:hover'], transform: 'translateY(-3px)'},
              '&:focus-visible': {outline: `2px solid ${ACCENTS.electric}`, outlineOffset: '2px'},
              display: 'flex',
              flexDirection: 'column',
              gap: 0.875,
              p: 2,
              borderRadius: '16px',
              color: 'inherit',
              textDecoration: 'none',
              '&:hover .cta': {textDecoration: 'underline', textUnderlineOffset: '3px'},
            }}
          >
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
              <Box
                sx={{
                  width: 36,
                  height: 36,
                  flex: 'none',
                  borderRadius: '11px',
                  display: 'grid',
                  placeItems: 'center',
                  color: '#fff',
                  background: BADGE_GRADIENT,
                  boxShadow: `0 8px 18px -8px color-mix(in srgb, ${ACCENTS.electric} 75%, transparent)`,
                }}
              >
                <Icon size={18} strokeWidth={2} aria-hidden />
              </Box>
              <Typography
                component="h3"
                sx={{
                  fontFamily: DISPLAY_FONT,
                  fontSize: '1rem',
                  fontWeight: 700,
                  letterSpacing: '-0.01em',
                  lineHeight: 1.3,
                  color: 'text.primary',
                  m: 0,
                }}
              >
                {name(title)}
              </Typography>
            </Box>
            <Typography sx={{color: 'text.secondary', fontSize: '0.8125rem', lineHeight: 1.5}}>{name(text)}</Typography>
            <Box
              component="span"
              className="cta"
              sx={{
                mt: 'auto',
                display: 'inline-flex',
                alignItems: 'center',
                gap: 0.75,
                color: linkColor(t),
                fontWeight: 600,
                fontSize: '0.875rem',
              }}
            >
              {cta}
              <ExternalLink size={14} strokeWidth={2} aria-hidden />
              <Box component="span" sx={VISUALLY_HIDDEN_SX}>
                (opens in a new tab)
              </Box>
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/* ─── Left panel: identity, fixed beside the feed on desktop ───────────── */

function IdentityPanel(): JSX.Element {
  const t = useHomeTokens();
  const productName = useProductName();
  return (
    <Box
      sx={{
        position: {md: 'sticky'},
        top: {md: 'var(--ifm-navbar-height)'},
        height: {md: 'calc(100vh - var(--ifm-navbar-height))'},
        display: 'flex',
        flexDirection: 'column',
        bgcolor: SPLIT_PANE_BG[t.isDark ? 'dark' : 'light'],
        borderRight: {md: '1px solid'},
        borderBottom: {xs: '1px solid', md: 'none'},
        borderColor: {xs: t.line, md: t.line},
        // On short screens the panel scrolls rather than cutting off its content.
        overflowX: 'hidden',
        overflowY: {md: 'auto'},
      }}
    >
      {/* The intro sits in the space above the get-involved cards, centred vertically on desktop. */}
      <Box
        component="header"
        sx={{
          flex: '1 0 auto',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'center',
          px: {xs: 3, sm: 5, lg: 7},
          pt: {xs: 6, md: 6},
          pb: {xs: 5, md: 6},
        }}
      >
        <Typography
          component="p"
          sx={{
            ...MONO_LABEL_SX,
            display: 'inline-flex',
            alignItems: 'center',
            gap: 1.25,
            color: linkColor(t),
            m: 0,
            mb: 1.5,
            '&::before': {content: '""', width: 20, height: '1px', bgcolor: 'currentColor'},
          }}
        >
          Events
        </Typography>
        <Typography
          variant="h1"
          sx={{
            fontFamily: DISPLAY_FONT,
            fontSize: {xs: '2.75rem', sm: '3.25rem', md: '3.25rem', xl: '3.75rem'},
            fontWeight: 700,
            lineHeight: 1.05,
            letterSpacing: '-0.035em',
            color: 'text.primary',
            m: 0,
          }}
        >
          {productName}
          <br />
          <Box component="span" sx={gradientTextSx}>
            Events
          </Box>
        </Typography>
        <Typography
          sx={{
            fontSize: {xs: '1.0625rem', xl: '1.125rem'},
            lineHeight: 1.6,
            color: 'text.secondary',
            mt: 1.75,
            maxWidth: 480,
          }}
        >
          Community days, conferences, and talks. Meet the maintainers, see {productName} live, or bring it to your
          city.
        </Typography>
        <Button
          component="a"
          href={`#${UPCOMING_ID}`}
          onClick={jumpTo(UPCOMING_ID)}
          variant="contained"
          color="primary"
          size="large"
          sx={{...primaryButtonSx, height: 48, px: 3, mt: 3, alignSelf: {sm: 'flex-start'}}}
        >
          See upcoming events
        </Button>
      </Box>
      <GetInvolved />
    </Box>
  );
}

/* ─── Right panel: the event feed ──────────────────────────────────────── */

/** A section of the feed. With an `id` it is a jump target for in-page links, which also move focus to it. */
function FeedSection({
  id = undefined,
  eyebrow,
  title,
  intro,
  children,
}: {
  id?: string;
  eyebrow: string;
  title: ReactNode;
  intro: ReactNode;
  children: ReactNode;
}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      component="section"
      id={id}
      tabIndex={id ? -1 : undefined}
      sx={{
        py: {xs: 6, md: 7},
        borderTop: '1px solid',
        borderColor: t.line,
        scrollMarginTop: 'var(--ifm-navbar-height)',
        '&:focus': {outline: 'none'},
      }}
    >
      <SectionTitle eyebrow={eyebrow} flush>
        {title}
      </SectionTitle>
      <Typography sx={INTRO_SX}>{intro}</Typography>
      {children}
    </Box>
  );
}

/* ─── Feed: featured flagship event ─────────────────────────────────────── */

function FeaturedFact({
  icon: Icon,
  label,
  value,
  note,
}: {
  icon: LucideIcon;
  label: string;
  value: string;
  note: string;
}): JSX.Element {
  return (
    <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
      <Box
        sx={{
          width: 44,
          height: 44,
          flex: 'none',
          borderRadius: '13px',
          display: 'grid',
          placeItems: 'center',
          color: '#fff',
          background: BADGE_GRADIENT,
          boxShadow:
            '0 10px 22px -10px color-mix(in srgb, #3688FF 75%, transparent), inset 0 1px 0 rgba(255,255,255,0.35)',
        }}
      >
        <Icon size={20} strokeWidth={2} aria-hidden />
      </Box>
      <Box>
        <Box component="span" sx={VISUALLY_HIDDEN_SX}>
          {label}:{' '}
        </Box>
        <Typography
          component="div"
          sx={{
            fontFamily: DISPLAY_FONT,
            fontWeight: 700,
            fontSize: '1.0625rem',
            letterSpacing: '-0.015em',
            lineHeight: 1.3,
            color: 'text.primary',
            ...TABULAR_SX,
          }}
        >
          {value}
        </Typography>
        <Typography component="div" sx={{fontSize: '0.875rem', color: 'text.secondary'}}>
          {note}
        </Typography>
      </Box>
    </Box>
  );
}

function Featured({event}: {event: HubEvent}): JSX.Element {
  const t = useHomeTokens();
  const blue = ACCENTS.electric;
  const surface = t.isDark ? 'rgb(6,30,58)' : t.surfaceStrong;
  return (
    <Box component="section" aria-labelledby="featured-event" sx={{pt: {xs: 5, md: 7}, pb: {xs: 6, md: 7}}}>
      <Typography component="p" sx={{...MONO_LABEL_SX, m: 0, mb: 1.75}}>
        Next flagship event
      </Typography>
      <Box
        sx={{
          position: 'relative',
          overflow: 'hidden',
          borderRadius: '20px',
          p: {xs: 3, md: 4},
          border: '1px solid transparent',
          background: `linear-gradient(150deg, color-mix(in srgb, ${blue} ${t.isDark ? 18 : 10}%, ${surface}), ${surface} 60%) padding-box, linear-gradient(135deg, ${blue}, color-mix(in srgb, ${blue} 25%, transparent) 55%, color-mix(in srgb, ${blue} 60%, transparent)) border-box`,
          boxShadow: t.shadow,
          // Dot grid fading out from the top-right corner.
          '&::after': {
            content: '""',
            position: 'absolute',
            inset: 0,
            ...DOT_GRID(t.isDark ? 'rgba(160,200,255,0.14)' : 'rgba(54,136,255,0.18)'),
            maskImage: 'radial-gradient(circle at 100% 0%, #000, transparent 45%)',
            WebkitMaskImage: 'radial-gradient(circle at 100% 0%, #000, transparent 45%)',
            pointerEvents: 'none',
          },
        }}
      >
        <TypeBadge type="hosted" />
        <Typography
          id="featured-event"
          variant="h2"
          sx={{
            position: 'relative',
            zIndex: 1,
            fontFamily: DISPLAY_FONT,
            fontSize: {xs: '1.625rem', md: '2rem'},
            fontWeight: 700,
            letterSpacing: '-0.025em',
            lineHeight: 1.15,
            mt: 2,
            mb: 1.25,
            '& a': {color: 'text.primary', textDecoration: 'none'},
            '& a:hover': {color: 'primary.main', textDecoration: 'none'},
          }}
        >
          <Link to={event.url}>{event.title}</Link>
        </Typography>
        <Typography sx={{color: 'text.secondary', fontSize: '1rem', lineHeight: 1.6}}>{event.summary}</Typography>
        <Box
          sx={{
            display: 'flex',
            flexDirection: {xs: 'column', sm: 'row'},
            flexWrap: 'wrap',
            gap: {xs: 2, sm: 4},
            mt: 3,
            pt: 3,
            borderTop: '1px solid',
            borderColor: t.line,
          }}
        >
          <FeaturedFact
            icon={Calendar}
            label="Date"
            value={formatFullDate(event.startDate, event.endDate)}
            note={formatWeekdays(event.startDate, event.endDate)}
          />
          <FeaturedFact
            icon={event.online ? Globe : MapPin}
            label="Location"
            value={event.online ? 'Online' : (event.city ?? '')}
            note={event.online ? 'Join from anywhere' : (event.country ?? '')}
          />
        </Box>
        <Button
          component={Link}
          to={event.url}
          variant="contained"
          color="primary"
          size="large"
          endIcon={<ArrowRight size={17} strokeWidth={2} aria-hidden />}
          sx={{...primaryButtonSx, height: 48, mt: 3.5}}
        >
          View event
        </Button>
      </Box>
    </Box>
  );
}

/* ─── Feed: upcoming timeline ──────────────────────────────────────────── */

/** Distance from the top of an entry to the centre of its timeline dot, aligned with the card's first line. */
const DOT_TOP = 30;
const DATE_COL = 64;
const RAIL_COL = 20;
const COL_GAP = 16;

function TimelineDot({type}: {type: EventType}): JSX.Element {
  const t = useHomeTokens();
  const blue = ACCENTS.electric;
  const dot = {
    hosted: {
      size: 16,
      background: BADGE_GRADIENT,
      border: 'none',
      boxShadow: `0 0 0 5px color-mix(in srgb, ${blue} ${t.isDark ? 25 : 18}%, transparent)`,
    },
    presence: {size: 12, background: t.isDark ? '#061a33' : '#fff', border: `2px solid ${blue}`, boxShadow: 'none'},
    talk: {
      size: 10,
      background: t.isDark ? 'rgba(160,200,255,0.45)' : 'rgba(15,40,80,0.35)',
      border: 'none',
      boxShadow: 'none',
    },
  }[type];
  return (
    <Box
      sx={{
        position: 'absolute',
        top: DOT_TOP - dot.size / 2,
        left: '50%',
        transform: 'translateX(-50%)',
        width: dot.size,
        height: dot.size,
        borderRadius: '50%',
        background: dot.background,
        border: dot.border,
        boxShadow: dot.boxShadow,
      }}
    />
  );
}

function Timeline({events}: {events: HubEvent[]}): JSX.Element {
  const t = useHomeTokens();
  const months = groupBy(events, (e) => e.startDate.slice(0, 7));
  return (
    <Box sx={{display: 'grid', gap: {xs: 4, md: 5}}}>
      {months.map((month) => (
        <Box key={month.key}>
          <Typography component="h3" sx={{...GROUP_HEADING_SX, pl: {sm: `${DATE_COL + RAIL_COL + COL_GAP * 2}px`}}}>
            {formatMonth(month.items[0].startDate)}
          </Typography>
          <Box component="ol" sx={{listStyle: 'none', p: 0, m: 0}}>
            {month.items.map((event, i) => {
              const first = i === 0;
              const last = i === month.items.length - 1;
              return (
                <Box
                  component="li"
                  key={event.id}
                  sx={{
                    display: 'grid',
                    gridTemplateColumns: {xs: 'minmax(0,1fr)', sm: `${DATE_COL}px ${RAIL_COL}px minmax(0,1fr)`},
                    columnGap: `${COL_GAP}px`,
                  }}
                >
                  <Typography
                    component="time"
                    dateTime={event.startDate}
                    sx={{
                      display: 'block',
                      pt: {sm: `${DOT_TOP - 13}px`},
                      mb: {xs: 1, sm: 0},
                      textAlign: {sm: 'right'},
                      fontFamily: DISPLAY_FONT,
                      fontWeight: 700,
                      fontSize: {xs: '0.9375rem', sm: '1rem'},
                      color: event.type === 'hosted' ? linkColor(t) : 'text.primary',
                      whiteSpace: 'nowrap',
                      ...TABULAR_SX,
                    }}
                  >
                    {formatDayRange(event.startDate, event.endDate)}
                  </Typography>
                  {/* Rail: a line through the month with a dot per event. */}
                  <Box
                    aria-hidden
                    sx={{
                      display: {xs: 'none', sm: 'block'},
                      position: 'relative',
                      '&::before': {
                        content: '""',
                        position: 'absolute',
                        left: '50%',
                        width: '2px',
                        ml: '-1px',
                        top: first ? DOT_TOP : 0,
                        bottom: last ? `calc(100% - ${DOT_TOP}px)` : 0,
                        background: `color-mix(in srgb, ${ACCENTS.electric} ${t.isDark ? 35 : 28}%, transparent)`,
                      },
                    }}
                  >
                    <TimelineDot type={event.type} />
                  </Box>
                  <Box sx={{pb: last ? 0 : 2, minWidth: 0}}>
                    <EventCard event={event} />
                  </Box>
                </Box>
              );
            })}
          </Box>
        </Box>
      ))}
    </Box>
  );
}

function Upcoming({events}: {events: HubEvent[]}): JSX.Element {
  const t = useHomeTokens();
  const productName = useProductName();
  return (
    <FeedSection
      id={UPCOMING_ID}
      eyebrow="What's next"
      title="Upcoming events"
      intro={`Where to find ${productName} next.`}
    >
      {events.length > 0 ? (
        <Timeline events={events} />
      ) : (
        <Typography sx={{...surfaceCardSx(t), p: 3, fontSize: '1rem', color: 'text.primary'}}>
          New events are on the way.{' '}
          <Box
            component="a"
            href={`#${PROPOSE_ID}`}
            onClick={jumpTo(PROPOSE_ID)}
            sx={{color: linkColor(t), fontWeight: 600}}
          >
            Want {productName} at yours?
          </Box>
        </Typography>
      )}
    </FeedSection>
  );
}

/* ─── Feed: past events ────────────────────────────────────────────────── */

function PastRow({event}: {event: HubEvent}): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box
      component="li"
      sx={{
        display: 'grid',
        gridTemplateColumns: {xs: 'minmax(0,1fr)', sm: `${DATE_COL}px minmax(0,1fr)`},
        columnGap: `${COL_GAP * 2 + RAIL_COL}px`,
        rowGap: 0.5,
        py: 2,
        borderTop: '1px solid',
        borderColor: t.line,
      }}
    >
      <Typography
        component="time"
        dateTime={event.startDate}
        sx={{
          fontSize: '0.9375rem',
          fontWeight: 600,
          color: 'text.secondary',
          whiteSpace: 'nowrap',
          textAlign: {sm: 'right'},
          pt: {sm: '2px'},
          ...TABULAR_SX,
        }}
      >
        {formatDayRange(event.startDate, event.endDate)}
      </Typography>
      <Box sx={{minWidth: 0}}>
        <EventTitle event={event} />
        <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', columnGap: 2, rowGap: 0.75, mt: 0.75}}>
          <TypeBadge type={event.type} />
          <Place event={event} />
        </Box>
      </Box>
    </Box>
  );
}

function PastYear({year, events}: {year: string; events: HubEvent[]}): JSX.Element {
  const hosted = events.filter((e) => e.type === 'hosted');
  const others = events.filter((e) => e.type !== 'hosted');
  return (
    <Box>
      <Typography component="h3" sx={GROUP_HEADING_SX}>
        {year}
      </Typography>
      {hosted.length > 0 && (
        <Box sx={{display: 'grid', gap: 2, mb: others.length ? 3 : 0}}>
          {hosted.map((event) => (
            <EventCard key={event.id} event={event} muted />
          ))}
        </Box>
      )}
      {others.length > 0 && (
        <Box component="ul" sx={{listStyle: 'none', p: 0, m: 0}}>
          {others.map((event) => (
            <PastRow key={event.id} event={event} />
          ))}
        </Box>
      )}
    </Box>
  );
}

function Past({events}: {events: HubEvent[]}): JSX.Element | null {
  const t = useHomeTokens();
  const productName = useProductName();
  const [expanded, setExpanded] = useState(false);
  const earlierId = useId();
  if (events.length === 0) return null;

  // Newest year first; within a year hosted, then presence, then talk, each newest first.
  const sorted = [...events].sort(
    (a, b) =>
      b.startDate.slice(0, 4).localeCompare(a.startDate.slice(0, 4)) ||
      TYPE_RANK[a.type] - TYPE_RANK[b.type] ||
      b.startDate.localeCompare(a.startDate),
  );
  const [latest, ...earlier] = groupBy(sorted, (e) => e.startDate.slice(0, 4));

  return (
    <FeedSection eyebrow="Archive" title="Past events" intro={`Where ${productName} has been.`}>
      <PastYear year={latest.key} events={latest.items} />
      {earlier.length > 0 && (
        <>
          {/* Earlier years stay in the page markup for search engines; the button only shows or hides them. */}
          <Box id={earlierId} hidden={!expanded} sx={{mt: 5, gap: 5, '&:not([hidden])': {display: 'grid'}}}>
            {earlier.map((group) => (
              <PastYear key={group.key} year={group.key} events={group.items} />
            ))}
          </Box>
          <Button
            variant="outlined"
            color="primary"
            aria-expanded={expanded}
            aria-controls={earlierId}
            onClick={() => setExpanded((v) => !v)}
            endIcon={
              <ChevronDown
                size={18}
                strokeWidth={2}
                aria-hidden
                style={{transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform 0.2s'}}
              />
            }
            sx={{...outlinedButtonSx(t), mt: 4, px: 2.5}}
          >
            {expanded ? 'Hide earlier years' : 'Show earlier years'}
          </Button>
        </>
      )}
    </FeedSection>
  );
}

/* ─── Page ───────────────────────────────────────────────────────────────── */

function StructuredData({hosted}: {hosted: HubEvent[]}): JSX.Element {
  const {withBaseUrl} = useBaseUrlUtils();
  const productName = useProductName();
  const data = {
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: `Events hosted by ${productName}`,
    itemListElement: hosted.map((event, i) => ({
      '@type': 'ListItem',
      position: i + 1,
      name: event.title,
      url: withBaseUrl(event.url, {absolute: true}),
    })),
  };
  return (
    <Head>
      <script type="application/ld+json">{JSON.stringify(data)}</script>
    </Head>
  );
}

export default function EventsHub(): JSX.Element {
  const productName = useProductName();
  const today = useToday();
  const name = (s: string): string => s.replaceAll('{{ProductName}}', productName);
  const events = EVENTS.map((e) => ({...e, title: name(e.title), summary: name(e.summary)}));

  const upcoming = events
    .filter((e) => !isPast(e, today))
    .sort((a, b) => a.startDate.localeCompare(b.startDate) || TYPE_RANK[a.type] - TYPE_RANK[b.type]);
  const past = events.filter((e) => isPast(e, today));
  const featured = upcoming.find((e) => e.type === 'hosted');
  const hosted = events.filter((e) => e.type === 'hosted').sort((a, b) => a.startDate.localeCompare(b.startDate));

  const title = `${productName} Events: Community Days, Conferences, and Talks`;
  const description = `Find ${productName} community days, conference booths, and talks near you. See upcoming and past events, or invite ${productName} to your meetup or conference.`;

  return (
    <Layout title={title} description={description}>
      <Head>
        <title>{title}</title>
        <meta property="og:title" content={title} />
        <link rel="canonical" href={CANONICAL_URL} />
      </Head>
      <StructuredData hosted={hosted} />
      <Box component="main" sx={{display: 'grid', gridTemplateColumns: {xs: 'minmax(0,1fr)', md: '44% minmax(0,1fr)'}}}>
        <IdentityPanel />
        {/* Feed column; the first section's top border is dropped so it meets the panel cleanly. */}
        <Box
          sx={{
            minWidth: 0,
            px: {xs: 2, sm: 5, lg: 7},
            '& > section:first-of-type': {borderTop: 'none'},
          }}
        >
          <Box sx={{maxWidth: 720}}>
            {featured && <Featured event={featured} />}
            <Upcoming events={upcoming} />
            <Past events={past} />
          </Box>
        </Box>
      </Box>
    </Layout>
  );
}
