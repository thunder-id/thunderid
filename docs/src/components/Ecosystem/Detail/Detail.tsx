// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Chip, Typography} from '@wso2/oxygen-ui';
import {Check, Clock, Download, Package, Scale, Users} from '@wso2/oxygen-ui-icons-react';
import {ComponentType, JSX, useState} from 'react';
import EntryIcon from './EntryIcon';
import {CodeCard, Cta, Pill, RailHeading, TabStrip} from './primitives';
import Section from './sections';
import {DEFAULT_ACCENT, toneColour, useInk} from './theme';
import SdkQuickstartDownload from '../../SdkQuickstartDownload';
import {artifactOf, entryHref, originOf, packageLabel} from '../presentation';
import useEntryFacts from '../useEntryFacts';
import useEntryVersion from '../useEntryVersion';
import {useEcosystem} from '@site/src/hooks/useEcosystem';
import type {
  EcosystemEntry,
  EcosystemHeroAside,
  EcosystemMetaIcon,
  EcosystemMetaItem,
  EcosystemSection,
} from '@site/src/types/ecosystem';

const META_ICONS: Record<EcosystemMetaIcon, ComponentType<{size?: number; strokeWidth?: number}>> = {
  package: Package,
  licence: Scale,
  clock: Clock,
  download: Download,
  people: Users,
};

/** Install card beside the hero: tabbed package managers, or a fixed snippet. */
function HeroAside({aside, accent}: {aside: EcosystemHeroAside; accent: string}): JSX.Element {
  const ink = useInk();
  const [tab, setTab] = useState(0);
  const tabs = aside.tabs ?? [];
  const code = aside.type === 'install' ? tabs[tab]?.code : aside.code;

  return (
    <Box
      sx={{
        width: {xs: '100%', md: 340},
        flexShrink: 0,
        border: '1px solid',
        borderColor: ink(0.08, 0.08),
        bgcolor: ink(0.015, 0.025),
        borderRadius: '14px',
        overflow: 'hidden',
      }}
    >
      {aside.title && (
        <Box sx={{px: 2, py: 1.5, borderBottom: '1px solid', borderColor: ink(0.05, 0.05)}}>
          <Typography
            component="span"
            sx={{
              fontFamily: 'monospace',
              fontSize: '9.5px',
              letterSpacing: '0.12em',
              textTransform: 'uppercase',
              color: ink(0.42, 0.35),
            }}
          >
            {aside.title}
          </Typography>
        </Box>
      )}
      {tabs.length > 1 && (
        <Box sx={{px: 1.5, pt: 1.5}}>
          <TabStrip labels={tabs.map((t) => t.label)} active={tab} onSelect={setTab} accent={accent} />
        </Box>
      )}
      {code && (
        <Box sx={{p: 1.5}}>
          <CodeCard code={code} accent={accent} />
        </Box>
      )}
      {aside.footnote && (
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1,
            px: 2,
            py: 1.375,
            borderTop: '1px solid',
            borderColor: ink(0.05, 0.05),
            bgcolor: `color-mix(in srgb, ${accent} 5%, transparent)`,
          }}
        >
          <Box component="span" sx={{width: 5, height: 5, borderRadius: '50%', bgcolor: accent, flexShrink: 0}} />
          <Typography sx={{fontSize: '11.5px', color: ink(0.48, 0.45)}}>{aside.footnote}</Typography>
        </Box>
      )}
    </Box>
  );
}

function Hero({entry, accent, version = undefined}: {entry: EcosystemEntry; accent: string; version?: string}): JSX.Element {
  const ink = useInk();
  const facts = useEntryFacts(entry);
  const official = originOf(entry) === 'official';
  const hero = entry.hero;

  // Published date and download count are facts about the package rather than
  // copy, so they are derived here instead of being written into the registry.
  const metaItems: EcosystemMetaItem[] = [
    ...(hero?.meta ?? []),
    ...(facts.updated ? [{icon: 'clock' as const, label: facts.updated}] : []),
    ...(facts.downloads ? [{icon: 'download' as const, label: facts.downloads}] : []),
  ];

  return (
    <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 2.75, flexWrap: 'wrap'}}>
      <Box
        sx={{
          width: 64,
          height: 64,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: '16px',
          bgcolor: `color-mix(in srgb, ${accent} 8%, transparent)`,
          border: '1px solid',
          borderColor: `color-mix(in srgb, ${accent} 19%, transparent)`,
        }}
      >
        <EntryIcon name={entry.icon} size={34} />
      </Box>

      <Box sx={{flex: '1 1 460px', minWidth: 280}}>
        <Box sx={{display: 'flex', alignItems: 'center', gap: 1.25, flexWrap: 'wrap', mb: 1}}>
          <Typography
            component="h1"
            sx={{fontSize: '32px', fontWeight: 700, letterSpacing: '-0.035em', m: 0, color: 'text.primary'}}
          >
            {entry.name}
          </Typography>
          <Chip
            size="small"
            variant="outlined"
            label={official ? 'Official' : 'Community'}
            icon={official ? <Check size={10} strokeWidth={3} /> : <Users size={10} strokeWidth={2.4} />}
            sx={{
              height: 22,
              color: official ? '#8bf9fa' : '#fbbf24',
              borderColor: official ? '#8bf9fa' : 'rgba(251,191,36,0.6)',
              '& .MuiChip-icon': {color: 'inherit', ml: '6px'},
            }}
          />
          {version && (
            <Chip
              size="small"
              variant="outlined"
              label={version}
              sx={{height: 22, color: '#4ade80', borderColor: 'rgba(74,222,128,0.5)', fontFamily: 'monospace'}}
            />
          )}
          {hero?.badges?.map((badge) => (
            <Pill key={badge.label} label={badge.label} colour={toneColour(badge.tone, ink(0.5, 0.6))} filled />
          ))}
          {entry.status === 'beta' && <Pill label="Beta" colour={accent} filled />}
        </Box>

        <Typography sx={{fontSize: '15.5px', lineHeight: 1.6, color: ink(0.62, 0.55), maxWidth: 620, mb: 2.25}}>
          {hero?.description ?? entry.description}
        </Typography>

        {metaItems.length > 0 && (
          <Box sx={{display: 'flex', flexWrap: 'wrap', gap: '10px 26px', mb: 2.75}}>
            {metaItems.map((item) => {
              const MetaIcon = META_ICONS[item.icon];
              return (
                <Box
                  key={item.label}
                  sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75, fontSize: '12.5px', color: ink(0.45, 0.4)}}
                >
                  <MetaIcon size={13} strokeWidth={2} />
                  <Box component="span" sx={item.mono ? {fontFamily: 'monospace', color: ink(0.6, 0.6)} : undefined}>
                    {item.label}
                  </Box>
                </Box>
              );
            })}
          </Box>
        )}

        {hero?.ctas && (
          <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1.25}}>
            {hero.ctas.map((cta) => (
              <Cta key={cta.href} cta={cta} accent={accent} />
            ))}
          </Box>
        )}
      </Box>

      {hero?.aside && <HeroAside aside={hero.aside} accent={accent} />}
    </Box>
  );
}

/**
 * Trail back to the listing.
 *
 * Only the two real levels: an entry sits directly under the listing. The
 * design drew the platform ("SPA") between them, but that predates the split of
 * the single category axis into Type and Platform, and it was a dead segment
 * either way since no page groups entries by platform.
 */
function Breadcrumb({entry, trail = []}: {entry: EcosystemEntry; trail?: {label: string}[]}): JSX.Element {
  const ink = useInk();
  const linkSx = {
    color: 'inherit',
    textDecoration: 'none',
    transition: 'color 0.15s',
    '&:hover': {color: 'text.primary'},
  };
  const separator = (
    <Box component="span" sx={{opacity: 0.5}}>
      /
    </Box>
  );

  return (
    <Box
      component="nav"
      aria-label="Breadcrumb"
      sx={{display: 'flex', alignItems: 'center', gap: 1, mb: 3.25, fontSize: '12.5px', color: ink(0.4, 0.35)}}
    >
      <Box component={Link} to="/sdks" sx={linkSx}>
        SDKs &amp; Tools
      </Box>
      {separator}
      {trail.length === 0 ? (
        <Box component="span" sx={{color: ink(0.75, 0.8)}}>
          {entry.name}
        </Box>
      ) : (
        <Box component={Link} to={`/sdks/${entry.id}`} sx={linkSx}>
          {entry.name}
        </Box>
      )}
      {trail.map((crumb) => (
        <Box key={crumb.label} sx={{display: 'contents'}}>
          {separator}
          <Box component="span" sx={{color: ink(0.75, 0.8)}}>
            {crumb.label}
          </Box>
        </Box>
      ))}
    </Box>
  );
}

/** Sticky right rail: table of contents, package stats, and related entries. */
function Rail({
  entry,
  accent,
  sections,
}: {
  entry: EcosystemEntry;
  accent: string;
  sections: EcosystemSection[];
}): JSX.Element | null {
  const ink = useInk();
  const all = useEcosystem();
  const rail = entry.rail;
  const related = (rail?.related ?? [])
    .map((id) => all.find((candidate) => candidate.id === id))
    .filter((candidate): candidate is EcosystemEntry => candidate !== undefined);

  // `related` is derived from `rail`, so no rail means nothing to render here.
  const toc = sections;

  return (
    <Box
      component="aside"
      sx={{
        position: {md: 'sticky'},
        top: {md: 88},
        alignSelf: 'start',
        display: 'flex',
        flexDirection: 'column',
        gap: 3.25,
        minWidth: 0,
      }}
    >
      {toc.length > 0 && (
        <Box>
          <RailHeading>On this page</RailHeading>
          <Box
            component="nav"
            sx={{display: 'flex', flexDirection: 'column', borderLeft: '1px solid', borderColor: ink(0.08, 0.07)}}
          >
            {toc.map((section) => (
              <Box
                key={section.id}
                component="a"
                href={`#${section.id}`}
                sx={{
                  fontSize: '13px',
                  color: ink(0.5, 0.45),
                  textDecoration: 'none',
                  py: 0.75,
                  pl: 1.75,
                  transition: 'color 0.15s',
                  '&:hover': {color: 'text.primary'},
                }}
              >
                {section.title}
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {rail?.stats && rail.stats.length > 0 && (
        <Box>
          <RailHeading>{rail?.statsTitle ?? 'Package'}</RailHeading>
          <Box
            sx={{
              border: '1px solid',
              borderColor: ink(0.08, 0.07),
              bgcolor: ink(0.015, 0.02),
              borderRadius: '12px',
              px: 2,
              py: 1.5,
            }}
          >
            {rail.stats.map((stat) => (
              <Box
                key={stat.label}
                sx={{display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 1.25, py: 0.625}}
              >
                <Typography sx={{fontSize: '12.5px', color: ink(0.45, 0.42)}}>{stat.label}</Typography>
                <Typography sx={{fontFamily: 'monospace', fontSize: '12px', color: ink(0.7, 0.75)}}>
                  {stat.value}
                </Typography>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {related.length > 0 && (
        <Box>
          <RailHeading>{rail?.relatedTitle ?? 'Related'}</RailHeading>
          <Box sx={{display: 'flex', flexDirection: 'column', gap: 1}}>
            {related.map((item) => {
              const href = entryHref(item);
              const inner = (
                <>
                  <Box
                    sx={{
                      width: 26,
                      height: 26,
                      flexShrink: 0,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      borderRadius: '7px',
                      bgcolor: ink(0.03, 0.04),
                    }}
                  >
                    <EntryIcon name={item.icon} size={15} />
                  </Box>
                  <Box sx={{flex: 1, minWidth: 0}}>
                    <Typography sx={{fontSize: '12.5px', fontWeight: 500, color: 'text.primary'}}>
                      {item.name}
                    </Typography>
                    <Typography
                      sx={{
                        fontFamily: 'monospace',
                        fontSize: '10px',
                        color: ink(0.35, 0.35),
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {packageLabel(item)}
                    </Typography>
                  </Box>
                  <Typography
                    component="span"
                    sx={{
                      fontFamily: 'monospace',
                      fontSize: '9px',
                      letterSpacing: '0.08em',
                      textTransform: 'uppercase',
                      color: ink(0.35, 0.35),
                      flexShrink: 0,
                    }}
                  >
                    {artifactOf(item)}
                  </Typography>
                </>
              );
              const sx = {
                display: 'flex',
                alignItems: 'center',
                gap: 1.25,
                px: 1.5,
                py: 1.25,
                border: '1px solid',
                borderColor: ink(0.07, 0.06),
                bgcolor: ink(0.015, 0.02),
                borderRadius: '10px',
                textDecoration: 'none',
                transition: 'border-color 0.18s',
                '&:hover': {borderColor: `color-mix(in srgb, ${accent} 35%, transparent)`},
              };
              return href ? (
                <Box key={item.id} component={Link} to={href} sx={sx}>
                  {inner}
                </Box>
              ) : (
                <Box key={item.id} sx={sx}>
                  {inner}
                </Box>
              );
            })}
          </Box>
        </Box>
      )}
    </Box>
  );
}

/**
 * Renders an entry's detail page from its `entry.yaml`.
 *
 * Used from a doc page as `<SdkDetail id="react" />`. The surrounding docs
 * chrome supplies the breadcrumb, sidebar, and table of contents, so this
 * renders only the parts the registry owns: the hero, the ordered sections, and
 * the closing stats and related links. Anything genuinely specific to one SDK
 * stays as prose in the MDX body beneath it.
 */
/**
 * The page shell every SDK & tools page shares: a banded header, a main column
 * of sections, and a sticky rail.
 *
 * An entry's own page and each of its guides both render through this, so a
 * guide is visually a continuation of the entry rather than a different kind of
 * page. `sections` is what varies; everything around it does not.
 *
 * Doc links are used as authored rather than rewritten to the reader's version.
 * These routes are generated from the `current` registry, so they describe
 * `next`, and a released snapshot may not have the pages they point at.
 */
export function Shell({
  entry,
  sections,
  trail = [],
  lead = undefined,
  banner = undefined,
  apiHref = '',
}: {
  entry: EcosystemEntry;
  sections: EcosystemSection[];
  trail?: {label: string}[];
  lead?: {title: string; intro?: string; minutes?: number};
  /** Callout above everything else in the main column. */
  banner?: JSX.Element;
  /** Where an `api` section links for the full reference, when one exists. */
  apiHref?: string;
}): JSX.Element {
  const ink = useInk();
  const version = useEntryVersion(entry);
  const accent = entry.accent ?? DEFAULT_ACCENT;

  return (
    <>
      <Box
        component="header"
        sx={{
          borderBottom: '1px solid',
          borderColor: ink(0.08, 0.06),
          background: `radial-gradient(ellipse 70% 100% at 15% 0%, color-mix(in srgb, ${accent} 9%, transparent) 0%, transparent 65%)`,
        }}
      >
        <Box sx={{maxWidth: 1200, mx: 'auto', px: {xs: 2, sm: 4}, pt: 3.25, pb: 4.25}}>
          <Breadcrumb entry={entry} trail={trail} />
          <Hero entry={entry} accent={accent} version={version} />
        </Box>
      </Box>

      <Box
        sx={{
          maxWidth: 1200,
          width: '100%',
          mx: 'auto',
          px: {xs: 2, sm: 4},
          pt: {xs: 4, md: 5.5},
          pb: 10,
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: 'minmax(0, 1fr) 260px'},
          gap: {xs: 5, md: 7},
          alignItems: 'start',
        }}
      >
        <Box component="main" sx={{minWidth: 0, display: 'flex', flexDirection: 'column', gap: 6.5}}>
          {banner}
          {lead && (
            <Box>
              <Box sx={{display: 'flex', alignItems: 'baseline', gap: 1.5, mb: lead.intro ? 0.75 : 0}}>
                <Typography
                  component="h1"
                  sx={{fontSize: '24px', fontWeight: 600, letterSpacing: '-0.025em', m: 0, color: 'text.primary'}}
                >
                  {lead.title}
                </Typography>
                {lead.minutes !== undefined && (
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
                    ~{lead.minutes} min
                  </Typography>
                )}
              </Box>
              {lead.intro && (
                <Typography sx={{fontSize: '14px', lineHeight: 1.65, color: ink(0.6, 0.5), maxWidth: 660}}>
                  {lead.intro}
                </Typography>
              )}
            </Box>
          )}
          {sections.map((section) => (
            <Section
              key={section.id}
              section={section}
              accent={accent}
              docs={entry.docs ?? {}}
              entryId={entry.id}
              guides={entry.guides ?? []}
              apiHref={apiHref}
            />
          ))}
        </Box>
        <Rail entry={entry} accent={accent} sections={sections} />
      </Box>
    </>
  );
}

/**
 * An entry's own page: the shell over the entry's sections.
 *
 * The sample callout is the same one the quickstarts carry, keyed on the entry
 * id because that is what the release feed calls the package. It renders
 * nothing for an entry with no published sample archive, so entries opt in by
 * publishing one rather than by being listed here.
 */
export default function Detail({entry}: {entry: EcosystemEntry}): JSX.Element {
  return (
    <Shell
      entry={entry}
      sections={entry.sections ?? []}
      apiHref={`/sdks/${entry.id}/apis`}
      banner={
        // The callout carries its own bottom margin for prose pages; here the
        // column's own gap does that work.
        <Box sx={{'& > *': {mb: 0}}}>
          <SdkQuickstartDownload packageId={entry.id} icon={<EntryIcon name={entry.icon} size={22} />} />
        </Box>
      }
    />
  );
}
