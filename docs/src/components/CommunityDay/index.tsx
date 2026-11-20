// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Head from '@docusaurus/Head';
import Link from '@docusaurus/Link';
import {useLocation} from '@docusaurus/router';
import useBaseUrl, {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import ThemedImage from '@theme/ThemedImage';
import {Box, Button, ButtonBase, IconButton, Popover, Typography} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  Calendar,
  Check,
  ChevronRight,
  Clock,
  Coffee,
  GitHub,
  MapPin,
  Ticket,
  User,
  X,
} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useId, useState, type ComponentType, type JSX, type ReactNode} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {
  BADGE_GRADIENT,
  DOT_GRID,
  MONO_LABEL_SX,
  accentCardSx,
  primaryButtonSx,
  REDUCED_MOTION,
} from '@site/src/components/Events/eventsTheme';
import {Aurora, Band, Content, SectionTitle} from '@site/src/components/Events/shared';
import {
  ACCENTS,
  DISPLAY_FONT,
  gradientTextSx,
  useHomeTokens,
  type HomeTokens,
} from '@site/src/components/HomePage/homeTheme';
import {MaskLogo, Reveal, TextLink} from '@site/src/components/HomePage/shared';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

export interface CommunityDaySpeaker {
  name: string;
  /** GitHub username, used for the avatar and profile link. */
  github: string;
  /** Site path of a photo to use instead of the GitHub avatar. */
  photo?: string;
  /** Role or affiliation shown under the name. `{{ProductName}}` is replaced. */
  affiliation?: string;
  /** Short bio shown when the speaker chip is clicked. `{{ProductName}}` is replaced. */
  bio?: string;
}

export interface CommunityDaySession {
  title: string;
  /** Two-line summary of the talk, shown between the title and the speakers. `{{ProductName}}` is replaced. */
  abstract?: string;
  /** Leave empty while the speaker is not yet confirmed. */
  speakers: CommunityDaySpeaker[];
}

export interface CommunityDayPartner {
  name: string;
  /** Name of a monochrome SVG in `static/assets/images/home/logos`. Falls back to the name as text. */
  logo?: string;
  /** Rendered logo width in pixels at a 30px height. Defaults to 160. */
  logoWidth?: number;
  url?: string;
}

export interface CommunityDayEvent {
  /** Page and hero title. `{{ProductName}}` is replaced with the product name. */
  title: string;
  /** Highlighted last line of the hero title, e.g. "Colombo 2026". */
  edition: string;
  /** Short label used in the breadcrumb. */
  shortName: string;
  summary: string;
  /** Meta description for search results and link previews. */
  description: string;
  /** ISO 8601 date-times with offset, used for structured data. */
  startDate: string;
  endDate: string;
  dateLabel: string;
  /** Shown under the date, e.g. the weekday. */
  dateNote?: string;
  timeLabel: string;
  timeNote: string;
  venue: {
    name: string;
    address?: string;
    locality: string;
    /** ISO 3166-1 alpha-2 country code. */
    country: string;
    note: string;
  };
  cost: string;
  /** External registration form. While unset, the register buttons show that registration opens soon. */
  registrationUrl?: string;
  /** Partner shown above the product logo in the hero. */
  heroPartner?: CommunityDayPartner;
  /** Icon component, drawn both as the card badge and as the large corner art. */
  audience: {icon: ComponentType<{size?: number; strokeWidth?: number}>; title: string; description: string}[];
  takeaways: string[];
  sessions: CommunityDaySession[];
  /** Short line under the Sessions heading. `{{ProductName}}` is replaced. */
  sessionsIntro?: string;
  sessionsNote?: string;
  organizers: CommunityDayPartner[];
  partners: CommunityDayPartner[];
  sponsors: CommunityDayPartner[];
}

function RegisterButton({url = undefined}: {url?: string}): JSX.Element {
  const sx = primaryButtonSx;

  if (!url) {
    return (
      <Button
        variant="contained"
        color="primary"
        size="large"
        disabled
        startIcon={<Clock size={17} strokeWidth={2} />}
        sx={{
          ...sx,
          '&.Mui-disabled': {
            color: 'primary.main',
            bgcolor: 'color-mix(in srgb, #3688FF 10%, transparent)',
            border: '1px dashed color-mix(in srgb, #3688FF 45%, transparent)',
            boxShadow: 'none',
          },
        }}
      >
        Registration opens soon
      </Button>
    );
  }

  return (
    <Button
      component={Link}
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      variant="contained"
      color="primary"
      size="large"
      endIcon={<ArrowRight size={17} strokeWidth={2} />}
      sx={sx}
    >
      Register now
    </Button>
  );
}

function Fact({
  icon,
  label,
  value,
  note = undefined,
}: {
  icon: ReactNode;
  label: string;
  value: ReactNode;
  note?: ReactNode;
}): JSX.Element {
  const t = useHomeTokens();
  const accent = ACCENTS.electric;
  return (
    <Box
      sx={{
        position: 'relative',
        display: 'flex',
        flexDirection: {xs: 'row', md: 'column'},
        alignItems: 'flex-start',
        gap: {xs: 2, md: 2.25},
        px: {xs: 1.25, md: 3},
        py: {xs: 1.25, md: 1.75},
        borderRadius: '14px',
        transition: 'background 0.25s ease',
        '&:hover': {
          background: `radial-gradient(ellipse 90% 80% at 50% 0%, color-mix(in srgb, ${accent} 10%, transparent), transparent 75%)`,
        },
        '&:hover .fact-tile': {transform: 'translateY(-2px)'},
        // Fading vertical divider between columns on the single-row layout.
        '&:not(:first-of-type)::before': {
          content: '""',
          display: {xs: 'none', md: 'block'},
          position: 'absolute',
          left: 0,
          top: '8%',
          bottom: '8%',
          width: '1px',
          background: `linear-gradient(to bottom, transparent, ${t.lineStrong}, transparent)`,
        },
      }}
    >
      <Box
        className="fact-tile"
        sx={{
          width: 46,
          height: 46,
          flex: 'none',
          borderRadius: '14px',
          display: 'grid',
          placeItems: 'center',
          color: '#fff',
          background: BADGE_GRADIENT,
          boxShadow: `0 10px 22px -10px color-mix(in srgb, ${accent} 75%, transparent), inset 0 1px 0 rgba(255,255,255,0.35)`,
          transition: 'transform 0.25s ease',
          [REDUCED_MOTION]: {transition: 'none'},
        }}
      >
        {icon}
      </Box>
      <Box sx={{minWidth: 0}}>
        <Typography component="div" sx={{...MONO_LABEL_SX, mb: 0.75}}>
          {label}
        </Typography>
        <Typography
          component="div"
          sx={{
            fontFamily: DISPLAY_FONT,
            fontWeight: 700,
            fontSize: '1.125rem',
            letterSpacing: '-0.015em',
            color: 'text.primary',
            lineHeight: 1.25,
            whiteSpace: {md: 'nowrap'},
          }}
        >
          {value}
        </Typography>
        {note && (
          <Typography component="div" sx={{fontSize: '0.875rem', color: 'text.secondary', mt: 0.5}}>
            {note}
          </Typography>
        )}
      </Box>
    </Box>
  );
}

function Hero({event, title}: {event: CommunityDayEvent; title: string}): JSX.Element {
  const t = useHomeTokens();
  const {withBaseUrl} = useBaseUrlUtils();
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;

  return (
    <Box
      component="header"
      sx={{
        position: 'relative',
        overflow: 'hidden',
        pt: {xs: 3, md: 4},
        pb: {xs: 7, md: 10},
      }}
    >
      <Aurora />
      <Content>
        <Box
          component="nav"
          aria-label="Breadcrumb"
          sx={{
            position: 'relative',
            display: 'flex',
            alignItems: 'center',
            gap: 0.75,
            fontSize: '0.875rem',
            mb: {xs: 4, md: 6},
          }}
        >
          <Box
            component={Link}
            to="/events"
            sx={{
              color: 'text.secondary',
              textDecoration: 'none',
              '&:hover': {color: 'text.primary', textDecoration: 'none'},
            }}
          >
            Events
          </Box>
          <ChevronRight size={14} aria-hidden />
          <Box component="span" aria-current="page" sx={{color: 'text.primary'}}>
            {event.shortName}
          </Box>
        </Box>

        <Box
          sx={{
            position: 'relative',
            textAlign: 'center',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
          }}
        >
          {event.heroPartner?.logo && (
            <Box sx={{color: 'text.secondary', mb: 3, lineHeight: 0}}>
              <MaskLogo name={event.heroPartner.logo} width={250} height={44} label={event.heroPartner.name} />
            </Box>
          )}
          <ThemedImage
            sources={{
              light: withBaseUrl('/assets/images/logo.svg'),
              dark: withBaseUrl('/assets/images/logo-inverted.svg'),
            }}
            alt={`${productName} Logo`}
            style={{height: 68}}
          />
          <Typography
            variant="h1"
            sx={{
              fontFamily: DISPLAY_FONT,
              fontSize: {xs: '2.25rem', sm: '3rem', md: '3.75rem'},
              fontWeight: 700,
              lineHeight: 1.08,
              letterSpacing: '-0.03em',
              color: 'text.primary',
              textWrap: 'balance',
              maxWidth: 820,
              mt: {xs: 4, md: 5},
              mb: 0,
            }}
          >
            {title}
            <Box component="span" sx={{display: 'block', ...gradientTextSx}}>
              {event.edition}
            </Box>
          </Typography>
          <Typography
            sx={{
              mt: 2.5,
              maxWidth: 560,
              fontSize: {xs: '1.0625rem', md: '1.25rem'},
              lineHeight: 1.55,
              color: 'text.secondary',
            }}
          >
            {event.summary}
          </Typography>
        </Box>

        <Box
          sx={{
            position: 'relative',
            mt: {xs: 5, md: 6},
            px: {xs: 1.5, md: 1.25},
            py: {xs: 1.5, md: 1.75},
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr', md: 'repeat(4, 1fr)'},
            rowGap: {xs: 0.5, md: 0},
            columnGap: {xs: 1, md: 0},
            borderRadius: '20px',
            // Gradient border: an opaque surface on the padding box over a brand gradient on the border box.
            border: '1px solid transparent',
            background: `linear-gradient(${t.isDark ? '#071a33' : '#ffffff'}, ${t.isDark ? '#071a33' : '#ffffff'}) padding-box, linear-gradient(120deg, color-mix(in srgb, ${ACCENTS.electric} 55%, transparent), color-mix(in srgb, ${ACCENTS.electric} 15%, transparent) 50%, color-mix(in srgb, ${ACCENTS.electric} 45%, transparent)) border-box`,
            boxShadow: t.isDark
              ? '0 30px 80px -40px rgba(0,0,0,0.9), 0 0 60px -30px rgba(54,136,255,0.5)'
              : '0 30px 70px -36px rgba(30,70,160,0.35), 0 0 60px -34px rgba(54,136,255,0.45)',
          }}
        >
          <Fact icon={<Calendar size={22} />} label="Date" value={event.dateLabel} note={event.dateNote} />
          <Fact icon={<Clock size={22} />} label="Time" value={event.timeLabel} note={event.timeNote} />
          <Fact
            icon={<MapPin size={22} />}
            label="Venue"
            value={event.venue.name}
            note={
              <>
                {event.venue.address && (
                  <>
                    {event.venue.address}
                    <br />
                  </>
                )}
                {event.venue.locality} · {event.venue.note}
              </>
            }
          />
          <Fact icon={<Ticket size={22} />} label="Cost" value={event.cost} />
        </Box>

        <Box sx={{position: 'relative', display: 'flex', justifyContent: 'center', mt: {xs: 4, md: 5}}}>
          <RegisterButton url={event.registrationUrl} />
        </Box>
      </Content>
    </Box>
  );
}

function Audience({event}: {event: CommunityDayEvent}): JSX.Element {
  const t = useHomeTokens();
  const blue = ACCENTS.electric;
  return (
    <Band>
      <SectionTitle eyebrow="Who it's for">Is this for you?</SectionTitle>
      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'}, gap: {xs: 2, md: 2.5}}}>
        {event.audience.map(({icon: Icon, title, description}, i) => (
          <Reveal key={title} delay={i * 80} sx={{height: '100%'}}>
            <Box
              sx={{
                ...accentCardSx(t),
                height: '100%',
                p: {xs: 3, md: 4},
                // Dot grid fading out from the top-right corner, behind the large icon.
                '&::after': {
                  content: '""',
                  position: 'absolute',
                  inset: 0,
                  ...DOT_GRID(t.isDark ? 'rgba(160,200,255,0.14)' : 'rgba(54,136,255,0.18)'),
                  maskImage: 'radial-gradient(circle at 100% 0%, #000, transparent 50%)',
                  WebkitMaskImage: 'radial-gradient(circle at 100% 0%, #000, transparent 50%)',
                  pointerEvents: 'none',
                },
                '& .audience-art': {transition: 'transform 0.5s ease, opacity 0.5s ease'},
                '& .audience-badge': {transition: 'transform 0.35s ease, box-shadow 0.35s ease'},
                '&:hover .audience-art': {transform: 'rotate(-4deg) scale(1.06)', opacity: 1},
                '&:hover .audience-badge': {
                  transform: 'scale(1.06)',
                  boxShadow: `0 16px 36px -10px color-mix(in srgb, ${blue} 80%, transparent)`,
                },
                [REDUCED_MOTION]: {
                  ...accentCardSx(t)[REDUCED_MOTION],
                  '&:hover .audience-art, &:hover .audience-badge': {transform: 'none'},
                },
              }}
            >
              {/* Large outline of the card's icon in the top-right corner, beside the badge. */}
              <Box
                aria-hidden
                className="audience-art"
                sx={{
                  position: 'absolute',
                  right: {xs: -22, md: -18},
                  top: {xs: -26, md: -30},
                  color: `color-mix(in srgb, ${blue} ${t.isDark ? 30 : 22}%, transparent)`,
                  opacity: 0.85,
                  transform: 'rotate(-10deg)',
                  pointerEvents: 'none',
                  lineHeight: 0,
                  zIndex: 1,
                  '& svg': {width: {xs: 130, md: 180}, height: {xs: 130, md: 180}},
                }}
              >
                <Icon size={180} strokeWidth={1.1} />
              </Box>

              <Box
                className="audience-badge"
                sx={{
                  position: 'relative',
                  zIndex: 2,
                  width: 68,
                  height: 68,
                  borderRadius: '20px',
                  display: 'grid',
                  placeItems: 'center',
                  color: '#fff',
                  background: BADGE_GRADIENT,
                  boxShadow: `0 12px 28px -10px color-mix(in srgb, ${blue} 70%, transparent), inset 0 1px 0 rgba(255,255,255,0.35)`,
                  // Soft halo ring around the badge.
                  outline: `6px solid color-mix(in srgb, ${blue} ${t.isDark ? 18 : 12}%, transparent)`,
                }}
              >
                <Icon size={34} strokeWidth={1.75} aria-hidden />
              </Box>
              <Typography
                variant="h3"
                sx={{
                  position: 'relative',
                  zIndex: 2,
                  fontFamily: DISPLAY_FONT,
                  fontSize: {xs: '1.375rem', md: '1.625rem'},
                  fontWeight: 700,
                  lineHeight: 1.25,
                  letterSpacing: '-0.02em',
                  color: 'text.primary',
                  mt: 3.5,
                  mb: 1.25,
                }}
              >
                {title}
              </Typography>
              <Typography
                sx={{
                  position: 'relative',
                  zIndex: 2,
                  fontSize: '1.0625rem',
                  lineHeight: 1.6,
                  color: 'text.secondary',
                  maxWidth: 440,
                }}
              >
                {description}
              </Typography>
            </Box>
          </Reveal>
        ))}
      </Box>
      <Reveal
        delay={160}
        sx={{
          mt: {xs: 2, md: 2.5},
          p: {xs: 3, md: 4},
          borderRadius: '20px',
          bgcolor: 'color-mix(in srgb, #3688FF 7%, transparent)',
          border: '1px solid color-mix(in srgb, #3688FF 18%, transparent)',
        }}
      >
        <Typography
          variant="h3"
          sx={{
            fontFamily: DISPLAY_FONT,
            fontSize: {xs: '1.25rem', md: '1.375rem'},
            fontWeight: 700,
            letterSpacing: '-0.015em',
            color: 'text.primary',
            mt: 0,
            mb: 2.5,
          }}
        >
          What you&apos;ll take away
        </Typography>
        <Box
          component="ul"
          sx={{
            listStyle: 'none',
            p: 0,
            m: 0,
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'},
            columnGap: 5,
            rowGap: {xs: 1.5, md: 2},
          }}
        >
          {event.takeaways.map((item) => (
            <Box
              component="li"
              key={item}
              sx={{display: 'flex', gap: 1.5, color: 'text.secondary', fontSize: '0.9375rem', lineHeight: 1.6}}
            >
              <Box
                component="span"
                sx={{
                  flex: 'none',
                  width: 22,
                  height: 22,
                  mt: '1px',
                  borderRadius: '50%',
                  display: 'grid',
                  placeItems: 'center',
                  color: '#fff',
                  background: BADGE_GRADIENT,
                }}
              >
                <Check size={13} strokeWidth={3} aria-hidden />
              </Box>
              {item}
            </Box>
          ))}
        </Box>
      </Reveal>
    </Band>
  );
}

function chipSx(t: HomeTokens, dashed = false) {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: 1,
    pl: 0.5,
    pr: 1.75,
    py: 0.5,
    maxWidth: '100%',
    borderRadius: '999px',
    border: dashed ? '1px dashed' : '1px solid',
    borderColor: dashed ? t.lineStrong : 'color-mix(in srgb, #3688FF 22%, transparent)',
    bgcolor: dashed ? 'transparent' : 'color-mix(in srgb, #3688FF 6%, transparent)',
  } as const;
}

function SpeakerPhoto({speaker, size}: {speaker: CommunityDaySpeaker; size: number}): JSX.Element {
  const t = useHomeTokens();
  const photoUrl = useBaseUrl(speaker.photo ?? '');
  return (
    <Box
      component="img"
      src={speaker.photo ? photoUrl : `https://github.com/${speaker.github}.png?size=160`}
      alt=""
      loading="lazy"
      width={size}
      height={size}
      sx={{
        width: size,
        height: size,
        borderRadius: '50%',
        objectFit: 'cover',
        flex: 'none',
        boxShadow: `0 0 0 2px ${t.isDark ? '#0a1a33' : '#ffffff'}`,
      }}
    />
  );
}

/**
 * Speaker chip. Clicking it (or pressing Enter/Space) opens a popover with the speaker's bio and
 * session; it closes with the close button, Esc, or a click outside, returning focus to the chip.
 */
function Speaker({
  speaker,
  affiliation = undefined,
  bio = undefined,
  sessionTitle,
}: {
  speaker: CommunityDaySpeaker;
  affiliation?: string;
  bio?: string;
  sessionTitle: string;
}): JSX.Element {
  const t = useHomeTokens();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);
  const titleId = useId();
  const open = anchor !== null;
  const close = (): void => setAnchor(null);

  return (
    <>
      <ButtonBase
        onClick={(e) => setAnchor(e.currentTarget)}
        aria-haspopup="dialog"
        aria-expanded={open}
        sx={{
          ...chipSx(t),
          pr: 2,
          textAlign: 'left',
          fontFamily: 'inherit',
          transition: 'border-color 0.2s ease, background-color 0.2s ease, transform 0.2s ease',
          '&:hover, &[aria-expanded="true"]': {
            borderColor: 'primary.main',
            bgcolor: 'color-mix(in srgb, #3688FF 11%, transparent)',
          },
          '&:hover': {transform: 'translateY(-1px)'},
          '&.Mui-focusVisible': {outline: '2px solid', outlineColor: 'primary.main', outlineOffset: '2px'},
          [REDUCED_MOTION]: {'&:hover': {transform: 'none'}},
        }}
      >
        <SpeakerPhoto speaker={speaker} size={36} />
        <Box component="span" sx={{display: 'block', minWidth: 0}}>
          <Typography
            component="span"
            noWrap
            sx={{display: 'block', fontWeight: 600, fontSize: '0.875rem', lineHeight: 1.3, color: 'text.primary'}}
          >
            {speaker.name}
          </Typography>
          {affiliation && (
            <Typography
              component="span"
              noWrap
              sx={{display: 'block', fontSize: '0.75rem', lineHeight: 1.3, color: 'text.secondary'}}
            >
              {affiliation}
            </Typography>
          )}
        </Box>
      </ButtonBase>

      <Popover
        open={open}
        anchorEl={anchor}
        onClose={close}
        anchorOrigin={{vertical: 'bottom', horizontal: 'left'}}
        transformOrigin={{vertical: 'top', horizontal: 'left'}}
        slotProps={{
          paper: {
            role: 'dialog',
            'aria-labelledby': titleId,
            sx: {
              position: 'relative',
              mt: 1,
              width: 360,
              maxWidth: 'calc(100vw - 32px)',
              p: 2.5,
              borderRadius: '16px',
              border: '1px solid',
              borderColor: t.lineStrong,
              bgcolor: t.isDark ? '#071a33' : '#ffffff',
              backgroundImage: 'none',
              boxShadow: t.shadow,
            },
          },
        }}
      >
        <IconButton
          onClick={close}
          aria-label="Close"
          size="small"
          sx={{position: 'absolute', top: 10, right: 10, color: 'text.secondary'}}
        >
          <X size={16} />
        </IconButton>

        <Box sx={{display: 'flex', alignItems: 'center', gap: 1.75, pr: 4}}>
          <SpeakerPhoto speaker={speaker} size={56} />
          <Box sx={{minWidth: 0}}>
            <Typography
              id={titleId}
              component="h4"
              sx={{
                fontFamily: DISPLAY_FONT,
                fontSize: '1.0625rem',
                fontWeight: 700,
                lineHeight: 1.3,
                color: 'text.primary',
                m: 0,
              }}
            >
              {speaker.name}
            </Typography>
            {affiliation && (
              <Typography sx={{fontSize: '0.8125rem', color: 'text.secondary', mt: 0.25}}>{affiliation}</Typography>
            )}
          </Box>
        </Box>

        {bio && (
          <Typography sx={{fontSize: '0.875rem', lineHeight: 1.6, color: 'text.secondary', mt: 2}}>{bio}</Typography>
        )}

        <Box sx={{mt: 2, pt: 2, borderTop: '1px solid', borderColor: t.line}}>
          <Typography component="div" sx={{...MONO_LABEL_SX, fontSize: '0.625rem', mb: 0.75}}>
            Speaking on
          </Typography>
          <Typography sx={{fontSize: '0.9375rem', fontWeight: 600, lineHeight: 1.4, color: 'text.primary'}}>
            {sessionTitle}
          </Typography>
        </Box>

        <Box sx={{mt: 2}}>
          <TextLink href={`https://github.com/${speaker.github}`} sx={{fontSize: '0.875rem'}}>
            <Box component="span" sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75}}>
              <GitHub size={15} aria-hidden />
              GitHub profile
            </Box>
          </TextLink>
        </Box>
      </Popover>
    </>
  );
}

function SpeakerToBeAnnounced(): JSX.Element {
  const t = useHomeTokens();
  return (
    <Box sx={chipSx(t, true)}>
      <Box
        sx={{
          width: 32,
          height: 32,
          borderRadius: '50%',
          bgcolor: t.isDark ? 'rgba(255,255,255,0.05)' : 'rgba(15,40,80,0.05)',
          display: 'grid',
          placeItems: 'center',
          color: 'text.secondary',
          flex: 'none',
        }}
      >
        <User size={15} aria-hidden />
      </Box>
      <Typography noWrap sx={{fontSize: '0.875rem', color: 'text.secondary'}}>
        Speaker to be announced
      </Typography>
    </Box>
  );
}

/** Vertical padding of each agenda row, in theme spacing units (1 unit = 8px). */
const ROW_PY = 2.25;

function Sessions({event, replaceName}: {event: CommunityDayEvent; replaceName: (s: string) => string}): JSX.Element {
  const t = useHomeTokens();

  return (
    <Band tinted>
      <Box sx={{maxWidth: 560, mb: {xs: 3, md: 4}}}>
        <SectionTitle eyebrow="Agenda" flush>
          Sessions
        </SectionTitle>
        {event.sessionsIntro && (
          <Typography sx={{fontSize: '1rem', lineHeight: 1.6, color: 'text.secondary', mt: 1.25}}>
            {replaceName(event.sessionsIntro)}
          </Typography>
        )}
      </Box>

      {/* Agenda: one full-width row per session, speakers under the abstract. */}
      <Reveal
        sx={{
          position: 'relative',
          overflow: 'hidden',
          p: {xs: 1, md: 1.25},
          borderRadius: '20px',
          bgcolor: t.isDark ? 'rgba(5,33,63,0.55)' : t.surfaceStrong,
          border: '1px solid',
          borderColor: t.lineStrong,
          boxShadow: t.shadow,
          backdropFilter: 'blur(16px)',
          '&::before': {
            content: '""',
            position: 'absolute',
            top: 0,
            left: '10%',
            right: '10%',
            height: '1px',
            background: 'linear-gradient(90deg, transparent, rgba(54,136,255,0.55), transparent)',
          },
        }}
      >
        <Box component="ul" sx={{listStyle: 'none', p: 0, m: 0}}>
          {event.sessions.map((session, i) => (
            <Box
              component="li"
              key={session.title}
              sx={{
                display: 'grid',
                gridTemplateColumns: '22px minmax(0, 1fr)',
                columnGap: {xs: 1.25, md: 2},
                rowGap: 0,
                py: ROW_PY,
                px: {xs: 1.5, md: 2.25},
                borderRadius: '14px',
                transition: 'background-color 0.2s ease',
                '&:hover': {bgcolor: 'color-mix(in srgb, #3688FF 5%, transparent)'},
              }}
            >
              {/* Bullet spans both grid rows (talk, then speakers) so the timeline reaches the next bullet. */}
              <Box sx={{position: 'relative', gridRow: 'span 2'}} aria-hidden>
                {i < event.sessions.length - 1 && (
                  <Box
                    sx={{
                      position: 'absolute',
                      top: 13,
                      left: 5,
                      width: '2px',
                      height: (theme) => `calc(100% + ${theme.spacing(ROW_PY * 2)})`,
                      borderRadius: '2px',
                      background: 'linear-gradient(to bottom, rgba(54,136,255,0.5), rgba(54,136,255,0.18))',
                    }}
                  />
                )}
                <Box
                  sx={{
                    position: 'relative',
                    zIndex: 1,
                    mt: '7px',
                    width: 12,
                    height: 12,
                    borderRadius: '50%',
                    ...(session.speakers.length > 0
                      ? {
                          bgcolor: 'primary.main',
                          boxShadow:
                            '0 0 0 4px color-mix(in srgb, #3688FF 20%, transparent), 0 0 14px rgba(54,136,255,0.55)',
                        }
                      : {
                          bgcolor: t.isDark ? '#0a1a33' : '#ffffff',
                          border: '2px solid',
                          borderColor: 'primary.main',
                          boxShadow: '0 0 0 4px color-mix(in srgb, #3688FF 12%, transparent)',
                        }),
                  }}
                />
              </Box>
              <Box sx={{minWidth: 0}}>
                <Typography
                  variant="h3"
                  sx={{
                    fontFamily: DISPLAY_FONT,
                    fontSize: {xs: '1.0625rem', md: '1.125rem'},
                    fontWeight: 700,
                    letterSpacing: '-0.015em',
                    lineHeight: 1.35,
                    color: 'text.primary',
                    m: 0,
                  }}
                >
                  {replaceName(session.title)}
                </Typography>
                {session.abstract && (
                  <Typography
                    sx={{
                      fontSize: '0.9375rem',
                      lineHeight: 1.55,
                      color: 'text.secondary',
                      maxWidth: '78ch',
                      textWrap: 'pretty',
                      mt: 0.5,
                      mb: 0,
                    }}
                  >
                    {replaceName(session.abstract)}
                  </Typography>
                )}
              </Box>
              <Box
                sx={{
                  gridColumn: 2,
                  display: 'flex',
                  flexWrap: 'wrap',
                  alignItems: 'center',
                  gap: 1,
                  mt: 1.5,
                  minWidth: 0,
                }}
              >
                {session.speakers.length > 0 ? (
                  session.speakers.map((speaker) => (
                    <Speaker
                      key={speaker.github}
                      speaker={speaker}
                      affiliation={speaker.affiliation && replaceName(speaker.affiliation)}
                      bio={speaker.bio && replaceName(speaker.bio)}
                      sessionTitle={replaceName(session.title)}
                    />
                  ))
                ) : (
                  <SpeakerToBeAnnounced />
                )}
              </Box>
            </Box>
          ))}
        </Box>
      </Reveal>
      {event.sessionsNote && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1.25,
            mt: 2.5,
            color: 'text.secondary',
            fontSize: '0.9375rem',
          }}
        >
          <Box component="span" sx={{color: 'primary.main', display: 'inline-flex'}}>
            <Coffee size={18} aria-hidden />
          </Box>
          {event.sessionsNote}
        </Box>
      )}
    </Band>
  );
}

function PartnerMark({partner}: {partner: CommunityDayPartner}): JSX.Element {
  const content = partner.logo ? (
    <MaskLogo name={partner.logo} width={partner.logoWidth ?? 160} height={30} label={partner.name} />
  ) : (
    <Typography
      component="span"
      sx={{fontFamily: DISPLAY_FONT, fontWeight: 700, fontSize: '1rem', letterSpacing: '-0.01em', lineHeight: 1.25}}
    >
      {partner.name}
    </Typography>
  );
  const sx = {
    display: 'inline-flex',
    alignItems: 'center',
    minHeight: 36,
    color: 'text.secondary',
    textDecoration: 'none',
    transition: 'color 0.2s',
  } as const;

  if (!partner.url) {
    return <Box sx={sx}>{content}</Box>;
  }

  return (
    <Box
      component={Link}
      to={partner.url}
      target="_blank"
      rel="noopener noreferrer"
      sx={{...sx, '&:hover': {color: 'text.primary', textDecoration: 'none'}}}
    >
      {content}
    </Box>
  );
}

function PartnerGroup({label, partners}: {label: string; partners: CommunityDayPartner[]}): JSX.Element | null {
  const t = useHomeTokens();
  if (partners.length === 0) return null;
  return (
    <Box
      sx={{
        position: 'relative',
        px: {xs: 0, md: 4},
        '&:first-of-type': {pl: 0},
        // Fading vertical divider between groups on the single-row layout.
        '&:not(:first-of-type)::before': {
          content: '""',
          display: {xs: 'none', md: 'block'},
          position: 'absolute',
          left: 0,
          top: 0,
          bottom: 0,
          width: '1px',
          background: `linear-gradient(to bottom, transparent, ${t.lineStrong}, transparent)`,
        },
      }}
    >
      <Typography component="h3" sx={{...MONO_LABEL_SX, mt: 0, mb: 1.75}}>
        {label}
      </Typography>
      <Box sx={{display: 'flex', flexWrap: 'wrap', alignItems: 'center', columnGap: 3.5, rowGap: 1.5}}>
        {partners.map((partner) => (
          <PartnerMark key={partner.name} partner={partner} />
        ))}
      </Box>
    </Box>
  );
}

function Partners({event}: {event: CommunityDayEvent}): JSX.Element {
  return (
    <Band>
      <SectionTitle eyebrow="Community">Organizers and partners</SectionTitle>
      <Reveal
        sx={{
          display: 'flex',
          flexDirection: {xs: 'column', md: 'row'},
          alignItems: {xs: 'flex-start', md: 'stretch'},
          gap: {xs: 3.5, md: 0},
        }}
      >
        <PartnerGroup label="Organized by" partners={event.organizers} />
        <PartnerGroup label="Partners" partners={event.partners} />
        <PartnerGroup label="Sponsored by" partners={event.sponsors} />
      </Reveal>
    </Band>
  );
}

const COUNTDOWN_UNITS = [
  {label: 'Days', ms: 86_400_000, mod: Infinity},
  {label: 'Hrs', ms: 3_600_000, mod: 24},
  {label: 'Min', ms: 60_000, mod: 60},
  {label: 'Sec', ms: 1_000, mod: 60},
] as const;

/**
 * Live countdown to the event start. Renders `--` on the server and first client paint so
 * hydration matches, then ticks every second. Hidden once the event has started.
 */
function Countdown({start}: {start: string}): JSX.Element | null {
  const t = useHomeTokens();
  const [now, setNow] = useState<number | null>(null);

  useEffect(() => {
    const tick = (): void => setNow(Date.now());
    const first = window.setTimeout(tick, 0);
    const id = window.setInterval(tick, 1000);
    return () => {
      window.clearTimeout(first);
      window.clearInterval(id);
    };
  }, []);

  const remaining = now === null ? null : Date.parse(start) - now;
  if (remaining !== null && remaining <= 0) return null;

  return (
    <Box role="timer" aria-label="Time until the event starts">
      <Typography component="div" sx={{...MONO_LABEL_SX, mb: 1}}>
        Doors open in
      </Typography>
      <Box sx={{display: 'flex', gap: 1}}>
        {COUNTDOWN_UNITS.map((unit) => {
          const value = remaining === null ? null : Math.floor(remaining / unit.ms) % unit.mod;
          return (
            <Box
              key={unit.label}
              sx={{
                minWidth: 56,
                px: 1,
                py: 0.875,
                textAlign: 'center',
                borderRadius: '10px',
                border: '1px solid',
                borderColor: t.line,
                bgcolor: 'color-mix(in srgb, #3688FF 5%, transparent)',
              }}
            >
              <Typography
                component="div"
                sx={{
                  fontFamily: DISPLAY_FONT,
                  fontSize: '1.375rem',
                  fontWeight: 700,
                  lineHeight: 1.1,
                  color: 'text.primary',
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {value === null ? '--' : String(value).padStart(2, '0')}
              </Typography>
              <Typography
                component="div"
                sx={{...MONO_LABEL_SX, fontSize: '0.5625rem', letterSpacing: '0.12em', mt: 0.25}}
              >
                {unit.label}
              </Typography>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

function SaveYourSeat({event}: {event: CommunityDayEvent}): JSX.Element {
  const t = useHomeTokens();
  const docsUrl = useDocsUrl();
  const {siteConfig} = useDocusaurusContext();
  const githubUrl = (siteConfig.customFields?.product as DocusaurusProductConfig).project.source.github.url;

  return (
    <Band tinted>
      <Reveal
        sx={{
          position: 'relative',
          overflow: 'hidden',
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: '1fr auto'},
          alignItems: 'center',
          gap: {xs: 3.5, md: 6},
          px: {xs: 3, md: 5},
          py: {xs: 3.5, md: 4},
          borderRadius: '20px',
          bgcolor: t.isDark ? 'rgba(5,33,63,0.55)' : t.surfaceStrong,
          border: '1px solid',
          borderColor: t.lineStrong,
          boxShadow: t.shadow,
          backdropFilter: 'blur(16px)',
          '&::before': {
            content: '""',
            position: 'absolute',
            inset: 0,
            pointerEvents: 'none',
            background: 'radial-gradient(ellipse 45% 90% at 0% 50%, rgba(54,136,255,0.1), transparent 70%)',
          },
          '&::after': {
            content: '""',
            position: 'absolute',
            top: 0,
            left: '10%',
            right: '10%',
            height: '1px',
            background: 'linear-gradient(90deg, transparent, rgba(54,136,255,0.55), transparent)',
          },
        }}
      >
        <Box sx={{position: 'relative'}}>
          <Typography
            variant="h2"
            sx={{
              fontFamily: DISPLAY_FONT,
              fontSize: {xs: '1.625rem', md: '2rem'},
              fontWeight: 700,
              letterSpacing: '-0.025em',
              color: 'text.primary',
              mt: 0,
              mb: 1,
            }}
          >
            Save your seat
          </Typography>
          <Typography sx={{maxWidth: 420, fontSize: '1rem', lineHeight: 1.6, color: 'text.secondary'}}>
            Free. In person. Live demos. Seats are limited, so register now.
          </Typography>
          <Box sx={{display: 'flex', flexWrap: 'wrap', gap: {xs: 2, sm: 3.5}, mt: 2.5}}>
            <TextLink href={githubUrl}>
              <Box component="span" sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75}}>
                <GitHub size={16} aria-hidden />
                View on GitHub
              </Box>
            </TextLink>
            <TextLink href={docsUrl('/docs/next')}>Read the docs</TextLink>
          </Box>
        </Box>
        <Box
          sx={{
            position: 'relative',
            display: 'flex',
            flexDirection: 'column',
            alignItems: {xs: 'stretch', sm: 'flex-start'},
            gap: 2.5,
          }}
        >
          <Countdown start={event.startDate} />
          <RegisterButton url={event.registrationUrl} />
        </Box>
      </Reveal>
    </Band>
  );
}

function StructuredData({event, title}: {event: CommunityDayEvent; title: string}): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const {pathname} = useLocation();
  const imageUrl = useBaseUrl('/assets/images/og-image.png', {absolute: true});
  const toOrg = (p: CommunityDayPartner) => ({'@type': 'Organization', name: p.name, ...(p.url ? {url: p.url} : {})});
  const data = {
    '@context': 'https://schema.org',
    '@type': 'Event',
    name: `${title} ${event.edition}`,
    description: event.description,
    startDate: event.startDate,
    endDate: event.endDate,
    eventStatus: 'https://schema.org/EventScheduled',
    eventAttendanceMode: 'https://schema.org/OfflineEventAttendanceMode',
    isAccessibleForFree: true,
    location: {
      '@type': 'Place',
      name: event.venue.name,
      address: {
        '@type': 'PostalAddress',
        ...(event.venue.address ? {streetAddress: event.venue.address} : {}),
        addressLocality: event.venue.locality,
        addressCountry: event.venue.country,
      },
    },
    organizer: event.organizers.map(toOrg),
    ...(event.sponsors.length > 0 ? {sponsor: event.sponsors.map(toOrg)} : {}),
    ...(event.registrationUrl
      ? {
          offers: {
            '@type': 'Offer',
            url: event.registrationUrl,
            price: 0,
            priceCurrency: 'USD',
            availability: 'https://schema.org/InStock',
          },
        }
      : {}),
    performer: event.sessions.flatMap((s) => s.speakers).map((s) => ({'@type': 'Person', name: s.name})),
    image: imageUrl,
    url: `${siteConfig.url}${pathname}`,
  };

  return (
    <Head>
      <script type="application/ld+json">{JSON.stringify(data)}</script>
    </Head>
  );
}

export default function CommunityDayPage({event}: {event: CommunityDayEvent}): JSX.Element {
  const {siteConfig} = useDocusaurusContext();
  const productName = (siteConfig.customFields?.product as DocusaurusProductConfig).project.name;
  const replaceName = (s: string): string => s.replaceAll('{{ProductName}}', productName);
  const title = replaceName(event.title);
  const eventWithNames: CommunityDayEvent = {
    ...event,
    summary: replaceName(event.summary),
    description: replaceName(event.description),
    takeaways: event.takeaways.map(replaceName),
  };

  return (
    <Layout title={`${title} ${event.edition}`} description={eventWithNames.description}>
      <StructuredData event={eventWithNames} title={title} />
      <main>
        <Hero event={eventWithNames} title={title} />
        <Audience event={eventWithNames} />
        <Sessions event={eventWithNames} replaceName={replaceName} />
        <Partners event={eventWithNames} />
        <SaveYourSeat event={eventWithNames} />
      </main>
    </Layout>
  );
}
