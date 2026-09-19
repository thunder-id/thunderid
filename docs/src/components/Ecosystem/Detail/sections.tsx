// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Typography} from '@wso2/oxygen-ui';
import {
  ArrowRight,
  ArrowRightLeft,
  ArrowUpRight,
  BookOpen,
  Building2,
  Key,
  PenLine,
  Server,
  ShieldCheck,
  Terminal,
  Zap,
} from '@wso2/oxygen-ui-icons-react';
import {ComponentType, JSX, useState} from 'react';
import {CodeCard, Cta, Note, Pill, SectionHeading, TabStrip} from './primitives';
import StepShell from './StepShell';
import {toneColour, useInk} from './theme';
import type {
  EcosystemApiEntry,
  EcosystemGuide,
  EcosystemApiSection,
  EcosystemCommandsSection,
  EcosystemDocs,
  EcosystemGuidesSection,
  EcosystemLinkGridSection,
  EcosystemQuickstartSection,
  EcosystemSection,
  EcosystemStep,
  EcosystemStepsSection,
  EcosystemSupportSection,
  EcosystemTableSection,
} from '@site/src/types/ecosystem';

const GUIDE_ICONS: Record<NonNullable<EcosystemGuide['icon']>, ComponentType<{size?: number; strokeWidth?: number}>> = {
  shield: ShieldCheck,
  terminal: Terminal,
  server: Server,
  pen: PenLine,
  building: Building2,
  exchange: ArrowRightLeft,
  key: Key,
  book: BookOpen,
};

/**
 * Renders `backtick` spans in registry prose as inline code chips.
 *
 * Section copy is plain YAML, not MDX, so nothing else would turn a package
 * name or a filename into code. Splitting on the delimiter keeps the authoring
 * format familiar without pulling a Markdown renderer into the page.
 */
function Prose({text, sx = undefined}: {text: string; sx?: object}): JSX.Element {
  // Odd indices are the spans between backticks. The index is part of each key
  // because the same word can legitimately appear as both prose and code.
  const parts = text.split('`').map((value, index) => ({key: `${index}:${value}`, value, code: index % 2 === 1}));

  return (
    <Typography sx={sx}>
      {parts.map((part) =>
        part.code ? (
          <Box
            key={part.key}
            component="code"
            sx={{
              fontFamily: 'monospace',
              fontSize: '0.92em',
              color: '#8bf9fa',
              bgcolor: 'color-mix(in srgb, #8bf9fa 8%, transparent)',
              borderRadius: '4px',
              px: '5px',
              py: '1px',
            }}
          >
            {part.value}
          </Box>
        ) : (
          <Box key={part.key} component="span" sx={{color: 'inherit'}}>
            {part.value}
          </Box>
        ),
      )}
    </Typography>
  );
}

interface SectionProps<T> {
  section: T;
  accent: string;
}

/**
 * Sections whose target lives in the entry's `docs:` block rather than in the
 * section itself. The dispatcher resolves it, so a renderer never has to know
 * which `docs` key it came from.
 */
interface LinkedSectionProps<T> extends SectionProps<T> {
  href: string;
}

/** One numbered step: description, then any of code, a click path, or a note. */
function Step({step, index, total, accent}: {step: EcosystemStep; index: number; total: number; accent: string}): JSX.Element {
  const ink = useInk();

  return (
    <StepShell index={index} total={total} title={step.title} accent={accent}>
      {step.description && (
        <Typography sx={{fontSize: '13.5px', lineHeight: 1.65, color: ink(0.55, 0.47), maxWidth: 620, mb: 1.75}}>
          {step.description}
        </Typography>
      )}
      {step.code && <CodeCard code={step.code} accent={accent} />}
      {step.uiPath && (
        <Box
          sx={{
            border: '1px solid',
            borderColor: ink(0.08, 0.07),
            bgcolor: ink(0.015, 0.02),
            borderRadius: '11px',
            px: 2,
            py: 1.75,
          }}
        >
          {step.uiPath.map((entry, i) => (
            <Box key={entry} sx={{display: 'flex', alignItems: 'center', gap: 1.125, py: 0.625}}>
              <Typography
                component="span"
                sx={{fontFamily: 'monospace', fontSize: '10px', color: ink(0.3, 0.28), width: 14, flexShrink: 0}}
              >
                {String(i + 1).padStart(2, '0')}
              </Typography>
              <Typography sx={{fontSize: '13px', color: ink(0.7, 0.68)}}>{entry}</Typography>
            </Box>
          ))}
        </Box>
      )}
      {step.note && <Note accent={accent}>{step.note}</Note>}
    </StepShell>
  );
}

/**
 * Numbered walkthrough, optionally split across tabs when the same task has
 * several surfaces (a CLI, a desktop app, a web console).
 */
function StepsSection({section, accent}: SectionProps<EcosystemStepsSection>): JSX.Element {
  const ink = useInk();
  const [tab, setTab] = useState(0);
  const groups = section.tabs;
  const active = groups?.[tab];

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} meta={section.meta} intro={section.intro} />
      {groups && (
        <>
          <TabStrip labels={groups.map((g) => g.label)} active={tab} onSelect={setTab} accent={accent} />
          {active?.blurb && (
            <Typography sx={{fontSize: '12.5px', lineHeight: 1.6, color: ink(0.52, 0.47), maxWidth: 620, mt: 1, mb: 3.25}}>
              {active.blurb}
            </Typography>
          )}
        </>
      )}
      {/* Every panel stays mounted and the inactive ones are hidden, so the
          content of a router you have not selected is still findable by search
          and by the browser's find-in-page. Matches <Lang> elsewhere. */}
      <Box sx={{mt: groups && !active?.blurb ? 3.25 : 0}}>
        {(groups ?? [{label: '', steps: section.steps ?? []}]).map((group, groupIndex) => (
          <Box key={group.label} sx={{display: groups && groupIndex !== tab ? 'none' : 'block'}}>
            {group.steps.map((step, i) => (
              <Step key={step.title} step={step} index={i} total={group.steps.length} accent={accent} />
            ))}
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/** Row table: compatibility matrices, requirements, claim mappings. */
function TableSection({section, accent}: SectionProps<EcosystemTableSection>): JSX.Element {
  const ink = useInk();
  const columns = section.columns;
  const template = columns.map((_, i) => (i === columns.length - 1 ? 'minmax(0,1.2fr)' : 'minmax(0,1fr)')).join(' ');

  const cellSx = (style: string | undefined): object =>
    style === 'code'
      ? {fontFamily: 'monospace', fontSize: '12.5px', color: ink(0.6, 0.7)}
      : style === 'accent'
        ? {fontFamily: 'monospace', fontSize: '12.5px', color: accent}
        : style === 'muted'
          ? {fontSize: '12.5px', color: ink(0.45, 0.47)}
          : {fontSize: '13.5px', color: ink(0.7, 0.7)};

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} intro={section.intro} />
      <Box
        sx={{
          border: '1px solid',
          borderColor: ink(0.08, 0.07),
          borderRadius: '12px',
          overflow: 'hidden',
          bgcolor: ink(0.015, 0.02),
        }}
      >
        {section.showHeader && (
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: template,
              gap: 2,
              px: 2.25,
              py: 1.375,
              borderBottom: '1px solid',
              borderColor: ink(0.08, 0.07),
              bgcolor: ink(0.015, 0.02),
            }}
          >
            {columns.map((column, i) => (
              <Typography
                key={column.label ?? i}
                component="span"
                sx={{
                  fontFamily: 'monospace',
                  fontSize: '9.5px',
                  letterSpacing: '0.11em',
                  textTransform: 'uppercase',
                  color: ink(0.42, 0.32),
                }}
              >
                {column.label}
              </Typography>
            ))}
          </Box>
        )}
        {section.rows.map((row) => (
          <Box
            key={row.cells.join('|')}
            sx={{
              display: 'grid',
              gridTemplateColumns: row.badge ? `${template} auto` : template,
              gap: 2,
              alignItems: 'baseline',
              px: 2.25,
              py: 1.625,
              borderBottom: '1px solid',
              borderColor: ink(0.05, 0.05),
              '&:last-of-type': {borderBottom: 'none'},
            }}
          >
            {row.cells.map((cell, i) => (
              <Typography key={cell} component="span" sx={{minWidth: 0, overflowWrap: 'break-word', ...cellSx(columns[i]?.style)}}>
                {cell}
              </Typography>
            ))}
            {row.badge && <Pill label={row.badge.label} colour={toneColour(row.badge.tone, accent)} filled />}
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/** Slash-command or CLI reference, grouped by a coloured tag. */
function CommandsSection({section, accent}: SectionProps<EcosystemCommandsSection>): JSX.Element {
  const ink = useInk();

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} intro={section.intro} />
      <Box sx={{display: 'flex', flexDirection: 'column', gap: 1.25}}>
        {section.items.map((item) => (
          <Box
            key={item.command}
            sx={{
              display: 'flex',
              alignItems: 'center',
              flexWrap: 'wrap',
              gap: 1.75,
              px: 2,
              py: 1.75,
              border: '1px solid',
              borderColor: ink(0.08, 0.07),
              bgcolor: ink(0.015, 0.02),
              borderRadius: '12px',
              transition: 'border-color 0.18s',
              '&:hover': {borderColor: `color-mix(in srgb, ${accent} 32%, transparent)`},
            }}
          >
            {item.group && (
              <Pill label={item.group} colour={toneColour(section.groupTones?.[item.group], accent)} filled />
            )}
            <Typography
              component="code"
              sx={{fontFamily: 'monospace', fontSize: '13px', fontWeight: 500, color: 'text.primary', minWidth: 0}}
            >
              {item.command}
            </Typography>
            <Typography
              sx={{fontSize: '12.5px', color: ink(0.5, 0.47), flex: 1, minWidth: 180, textAlign: {sm: 'right'}}}
            >
              {item.description}
            </Typography>
          </Box>
        ))}
      </Box>
    </Box>
  );
}

/** Grid of outbound links, e.g. "Beyond this guide". */
function LinkGridSection({section, accent}: SectionProps<EcosystemLinkGridSection>): JSX.Element {
  const ink = useInk();

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} intro={section.intro} />
      <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 1.25}}>
        {section.items.map((item) => {
          const external = item.external ?? item.href.startsWith('http');
          const sx = {
            display: 'flex',
            alignItems: 'center',
            gap: 1.375,
            px: 1.75,
            py: 1.625,
            border: '1px solid',
            borderColor: ink(0.08, 0.07),
            bgcolor: ink(0.015, 0.02),
            borderRadius: '10px',
            textDecoration: 'none',
            transition: 'all 0.18s',
            '&:hover': {
              borderColor: `color-mix(in srgb, ${accent} 40%, transparent)`,
              bgcolor: `color-mix(in srgb, ${accent} 4%, transparent)`,
            },
          };
          const body = (
            <>
              <Typography sx={{fontSize: '12.5px', fontWeight: 500, color: ink(0.75, 0.8), flex: 1, minWidth: 0}}>
                {item.label}
              </Typography>
              <ArrowUpRight size={13} strokeWidth={2.2} color={ink(0.35, 0.35)} />
            </>
          );
          return external ? (
            <Box key={item.href} component="a" href={item.href} target="_blank" rel="noopener noreferrer" sx={sx}>
              {body}
            </Box>
          ) : (
            <Box key={item.href} component={Link} to={item.href} sx={sx}>
              {body}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

/** Grid of cards, one per guide, each linking to that guide's own page. */
function GuidesSection({
  section,
  accent,
  entryId,
  guides,
}: SectionProps<EcosystemGuidesSection> & {entryId: string; guides: EcosystemGuide[]}): JSX.Element {
  const ink = useInk();
  const order = section.items;
  const shown: EcosystemGuide[] = order
    ? order.flatMap((id) => guides.filter((guide) => guide.id === id))
    : guides;

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} intro={section.intro} />
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: shown.length === 1 ? 'minmax(0, 420px)' : 'repeat(auto-fill, minmax(260px, 1fr))',
          gap: 1.75,
        }}
      >
        {shown.map((guide) => {
          const GuideIcon = GUIDE_ICONS[guide.icon ?? 'book'];
          return (
            <Box
              key={guide.id}
              component={Link}
              to={`/sdks/${entryId}/guides/${guide.id}`}
              sx={{
                display: 'flex',
                flexDirection: 'column',
                gap: 1.25,
                minHeight: 168,
                px: 2.25,
                py: 2.25,
                border: '1px solid',
                borderColor: ink(0.08, 0.07),
                bgcolor: ink(0.015, 0.02),
                borderRadius: '12px',
                textDecoration: 'none',
                transition: 'all 0.18s',
                '&:hover': {
                  borderColor: `color-mix(in srgb, ${accent} 40%, transparent)`,
                  bgcolor: `color-mix(in srgb, ${accent} 4%, transparent)`,
                },
              }}
            >
              <Box
                sx={{
                  width: 32,
                  height: 32,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: '9px',
                  bgcolor: `color-mix(in srgb, ${accent} 10%, transparent)`,
                  border: '1px solid',
                  borderColor: `color-mix(in srgb, ${accent} 20%, transparent)`,
                  color: accent,
                }}
              >
                <GuideIcon size={15} strokeWidth={1.8} />
              </Box>
              <Typography sx={{fontSize: '13.5px', fontWeight: 600, letterSpacing: '-0.01em', color: 'text.primary'}}>
                {guide.title}
              </Typography>
              {guide.description && (
                <Typography sx={{fontSize: '12.5px', lineHeight: 1.6, color: ink(0.55, 0.55)}}>
                  {guide.description}
                </Typography>
              )}
              {guide.minutes !== undefined && (
                <Typography
                  component="span"
                  sx={{
                    mt: 'auto',
                    pt: 0.25,
                    fontFamily: 'monospace',
                    fontSize: '9.5px',
                    letterSpacing: '0.1em',
                    textTransform: 'uppercase',
                    color: ink(0.45, 0.45),
                  }}
                >
                  {guide.minutes} min read
                </Typography>
              )}
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

/** Prominent card linking to the canonical quickstart. */
function QuickstartSection({section, accent, href}: LinkedSectionProps<EcosystemQuickstartSection>): JSX.Element {
  const ink = useInk();

  return (
    <Box component="section">
      <SectionHeading id={section.id} title={section.title} intro={section.intro} />
      <Box
        component={Link}
        to={href}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 2.25,
          flexWrap: 'wrap',
          px: 2.75,
          py: 2.5,
          border: '1px solid',
          borderColor: `color-mix(in srgb, ${accent} 28%, transparent)`,
          background: `linear-gradient(135deg, color-mix(in srgb, ${accent} 8%, transparent) 0%, color-mix(in srgb, #8bf9fa 4%, transparent) 100%)`,
          borderRadius: '13px',
          textDecoration: 'none',
          transition: 'border-color 0.18s, transform 0.18s',
          '&:hover': {
            borderColor: `color-mix(in srgb, ${accent} 55%, transparent)`,
            transform: 'translateY(-1px)',
          },
        }}
      >
        <Box
          sx={{
            width: 42,
            height: 42,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderRadius: '11px',
            bgcolor: `color-mix(in srgb, ${accent} 12%, transparent)`,
            border: '1px solid',
            borderColor: `color-mix(in srgb, ${accent} 30%, transparent)`,
            color: '#8bf9fa',
          }}
        >
          <Zap size={20} strokeWidth={1.8} />
        </Box>
        <Box sx={{flex: 1, minWidth: 220}}>
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1.125, flexWrap: 'wrap', mb: 0.625}}>
            <Typography sx={{fontSize: '15px', fontWeight: 600, letterSpacing: '-0.01em', color: 'text.primary'}}>
              {section.label}
            </Typography>
            {section.minutes !== undefined && (
              <Typography
                component="span"
                sx={{
                  fontFamily: 'monospace',
                  fontSize: '9.5px',
                  letterSpacing: '0.09em',
                  textTransform: 'uppercase',
                  color: ink(0.5, 0.5),
                  border: '1px solid',
                  borderColor: ink(0.14, 0.14),
                  borderRadius: '5px',
                  px: 0.875,
                  py: '2px',
                }}
              >
                ~{section.minutes} min
              </Typography>
            )}
          </Box>
          <Typography
            sx={{fontFamily: 'monospace', fontSize: '11.5px', color: ink(0.5, 0.5), overflowWrap: 'break-word'}}
          >
            {section.path}
          </Typography>
        </Box>
        <Box
          component="span"
          sx={{display: 'inline-flex', alignItems: 'center', gap: 0.875, fontSize: '13px', fontWeight: 600, color: accent, flexShrink: 0}}
        >
          Open in docs
          <ArrowUpRight size={13} strokeWidth={2.4} />
        </Box>
      </Box>
      {section.outro && (
        <Typography sx={{fontSize: '12.5px', lineHeight: 1.62, color: ink(0.45, 0.42), maxWidth: 640, mt: 2}}>
          {section.outro}
        </Typography>
      )}
    </Box>
  );
}

/**
 * Two-pane API explorer: exports grouped in a left nav, the selected one
 * detailed on the right.
 *
 * Shows a curated selection rather than the package's whole surface. The full
 * reference is generated from the type declarations, and the header links to it
 * when the entry names one.
 */
function ApiSection({section, accent, href}: LinkedSectionProps<EcosystemApiSection>): JSX.Element {
  const ink = useInk();
  const [selected, setSelected] = useState(0);
  const entry = section.exports[Math.min(selected, section.exports.length - 1)];

  // Preserve authored order while collecting each group's members.
  const groups = section.exports.reduce<{name: string; items: {item: EcosystemApiEntry; index: number}[]}[]>(
    (acc, item, index) => {
      const bucket = acc.find((g) => g.name === item.group);
      if (bucket) bucket.items.push({item, index});
      else acc.push({name: item.group, items: [{item, index}]});
      return acc;
    },
    [],
  );

  const eyebrowSx = {
    fontFamily: 'monospace',
    fontSize: '9px',
    letterSpacing: '0.13em',
    textTransform: 'uppercase' as const,
    color: ink(0.45, 0.42),
  };

  return (
    <Box component="section">
      <Box
        sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 2, flexWrap: 'wrap', mb: 0.75}}
      >
        <Typography
          component="h2"
          id={section.id}
          sx={{fontSize: '20px', fontWeight: 600, letterSpacing: '-0.02em', m: 0, color: 'text.primary'}}
        >
          {section.title}
        </Typography>
        {href && (
          <Box
            component={Link}
            to={href}
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 0.75,
              fontSize: '12.5px',
              fontWeight: 500,
              color: accent,
              textDecoration: 'none',
            }}
          >
            Full reference
            <ArrowRight size={12} strokeWidth={2.5} />
          </Box>
        )}
      </Box>
      {section.intro && (
        <Prose text={section.intro} sx={{fontSize: '14px', lineHeight: 1.65, color: ink(0.6, 0.5), maxWidth: 640, mb: 2.75}} />
      )}

      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: '174px minmax(0, 1fr)'},
          border: '1px solid',
          borderColor: ink(0.08, 0.07),
          borderRadius: '12px',
          overflow: 'hidden',
          bgcolor: ink(0.015, 0.02),
        }}
      >
        <Box
          sx={{
            borderRight: {sm: '1px solid'},
            borderBottom: {xs: '1px solid', sm: 'none'},
            borderColor: ink(0.08, 0.07),
            bgcolor: ink(0.01, 0.015),
            py: 1,
            minWidth: 0,
          }}
        >
          {groups.map((group) => (
            <Box key={group.name}>
              <Typography component="div" sx={{...eyebrowSx, px: 1.75, pt: 1.375, pb: 0.625}}>
                {group.name}
              </Typography>
              {group.items.map(({item, index}) => {
                const active = index === selected;
                return (
                  <Box
                    key={item.name}
                    component="button"
                    type="button"
                    aria-current={active}
                    onClick={() => setSelected(index)}
                    sx={{
                      display: 'block',
                      width: '100%',
                      textAlign: 'left',
                      px: 1.625,
                      py: 0.75,
                      border: 'none',
                      borderLeft: '2px solid',
                      borderLeftColor: active ? accent : 'transparent',
                      bgcolor: active ? `color-mix(in srgb, ${accent} 12%, transparent)` : 'transparent',
                      color: active ? 'text.primary' : ink(0.6, 0.6),
                      cursor: 'pointer',
                      fontFamily: 'monospace',
                      fontSize: '11px',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      '&:hover': {color: 'text.primary'},
                    }}
                  >
                    {item.name}
                  </Box>
                );
              })}
            </Box>
          ))}
        </Box>

        <Box sx={{minWidth: 0, px: 2.5, py: 2.5}}>
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1.25, flexWrap: 'wrap', mb: 1.125}}>
            <Pill label={entry.kind} colour={toneColour(section.kindTones?.[entry.kind], accent)} filled />
            <Typography
              component="code"
              sx={{fontFamily: 'monospace', fontSize: '15px', fontWeight: 500, color: 'text.primary'}}
            >
              {entry.name}
            </Typography>
          </Box>
          <Prose
            text={entry.description}
            sx={{fontSize: '13.5px', lineHeight: 1.65, color: ink(0.6, 0.55), maxWidth: 600, mb: 2.25}}
          />

          {entry.signature && (
            <>
              <Typography component="div" sx={{...eyebrowSx, mb: 1}}>
                Signature
              </Typography>
              <CodeCard code={{content: entry.signature}} accent={accent} copyable={false} />
            </>
          )}

          {entry.rows && entry.rows.length > 0 && (
            <>
              <Typography component="div" sx={{...eyebrowSx, mt: entry.signature ? 2.5 : 0, mb: 1}}>
                {entry.rowsLabel ?? 'Fields'}
              </Typography>
              <Box
                sx={{
                  border: '1px solid',
                  borderColor: ink(0.08, 0.07),
                  borderRadius: '10px',
                  overflow: 'hidden',
                  bgcolor: ink(0.015, 0.02),
                }}
              >
                {entry.rows.map((row) => (
                  <Box
                    key={row.name}
                    sx={{
                      display: 'grid',
                      gridTemplateColumns: {xs: '1fr', sm: 'minmax(140px, 1fr) minmax(0, 1.55fr)'},
                      gap: {xs: 0.75, sm: 2.25},
                      px: 2,
                      py: 1.625,
                      borderBottom: '1px solid',
                      borderColor: ink(0.05, 0.05),
                      '&:last-of-type': {borderBottom: 'none'},
                    }}
                  >
                    <Box sx={{minWidth: 0, display: 'flex', flexDirection: 'column', gap: 0.625}}>
                      <Box sx={{display: 'flex', alignItems: 'center', gap: 0.875, flexWrap: 'wrap'}}>
                        <Typography
                          component="code"
                          sx={{fontFamily: 'monospace', fontSize: '12.5px', fontWeight: 500, color: 'text.primary'}}
                        >
                          {row.name}
                        </Typography>
                        {row.tag && <Pill label={row.tag} colour={ink(0.4, 0.4)} />}
                      </Box>
                      <Typography
                        component="code"
                        sx={{fontFamily: 'monospace', fontSize: '11.5px', color: '#8bf9fa', overflowWrap: 'break-word'}}
                      >
                        {row.type}
                      </Typography>
                    </Box>
                    {row.description && (
                      <Typography sx={{fontSize: '13px', lineHeight: 1.6, color: ink(0.55, 0.55), minWidth: 0}}>
                        {row.description}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Box>
            </>
          )}

          {entry.example && (
            <>
              <Typography component="div" sx={{...eyebrowSx, mt: 2.5, mb: 1}}>
                Example
              </Typography>
              <CodeCard code={{content: entry.example}} accent={accent} copyable={false} />
            </>
          )}
        </Box>
      </Box>
    </Box>
  );
}

/** Closing support block: one panel for what we own, two when ownership splits. */
function SupportSection({section, accent}: SectionProps<EcosystemSupportSection>): JSX.Element {
  const ink = useInk();

  const panel = (title: string, body: string, ctas: EcosystemSupportSection['ctas']): JSX.Element => (
    <Box
      key={title}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        px: 2.25,
        py: 2,
        border: '1px solid',
        borderColor: ink(0.08, 0.08),
        bgcolor: ink(0.02, 0.025),
        borderRadius: '11px',
      }}
    >
      <Box>
        <Typography sx={{fontSize: '13.5px', fontWeight: 600, color: 'text.primary', mb: 0.625}}>{title}</Typography>
        <Typography sx={{fontSize: '12.5px', lineHeight: 1.6, color: ink(0.5, 0.45)}}>{body}</Typography>
      </Box>
      {ctas && (
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1, mt: 'auto'}}>
          {ctas.map((cta) => (
            <Cta key={cta.href} cta={cta} accent={accent} />
          ))}
        </Box>
      )}
    </Box>
  );

  return (
    <Box
      component="section"
      sx={{
        px: 3.25,
        py: 3,
        border: '1px solid',
        borderColor: `color-mix(in srgb, ${accent} 22%, transparent)`,
        background: `linear-gradient(135deg, color-mix(in srgb, ${accent} 6%, transparent) 0%, transparent 100%)`,
        borderRadius: '14px',
      }}
    >
      {section.eyebrow && (
        <Typography
          component="div"
          sx={{
            fontFamily: 'monospace',
            fontSize: '9.5px',
            letterSpacing: '0.14em',
            textTransform: 'uppercase',
            color: accent,
            mb: 1.75,
          }}
        >
          {section.eyebrow}
        </Typography>
      )}
      <Typography
        component="h3"
        id={section.id}
        sx={{fontSize: '16px', fontWeight: 600, letterSpacing: '-0.015em', color: 'text.primary', m: 0, mb: 1}}
      >
        {section.title}
      </Typography>
      {section.body && (
        <Typography sx={{fontSize: '13.5px', lineHeight: 1.65, color: ink(0.5, 0.47), maxWidth: 620, mb: 2.25}}>
          {section.body}
        </Typography>
      )}
      {section.variant === 'split' && section.panels ? (
        <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: 1.5}}>
          {section.panels.map((p) => panel(p.title, p.body, p.ctas))}
        </Box>
      ) : (
        section.ctas && (
          <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1.25}}>
            {section.ctas.map((cta) => (
              <Cta key={cta.href} cta={cta} accent={accent} />
            ))}
          </Box>
        )
      )}
    </Box>
  );
}

/**
 * Dispatches a registry section to its renderer.
 *
 * `quickstart` reads its link target from the entry's `docs:` block. The build
 * rejects the section on an entry that does not set the key, so by the time
 * this runs the target is present.
 *
 * `api` gets its target from `apiHref`, which the caller leaves empty when the
 * section is not the entry's own: the plugin builds a reference page from
 * `entry.sections`, so an `api` section inside a guide has none to link to.
 */
export default function Section({
  section,
  accent,
  docs,
  entryId,
  guides,
  apiHref = '',
}: SectionProps<EcosystemSection> & {
  docs: EcosystemDocs;
  entryId: string;
  guides: EcosystemGuide[];
  apiHref?: string;
}): JSX.Element | null {
  switch (section.type) {
    case 'steps':
      return <StepsSection section={section} accent={accent} />;
    case 'table':
      return <TableSection section={section} accent={accent} />;
    case 'commands':
      return <CommandsSection section={section} accent={accent} />;
    case 'linkGrid':
      return <LinkGridSection section={section} accent={accent} />;
    case 'guides':
      return <GuidesSection section={section} accent={accent} entryId={entryId} guides={guides} />;
    case 'quickstart':
      return <QuickstartSection section={section} accent={accent} href={docs.quickstart ?? ''} />;
    case 'api':
      return <ApiSection section={section} accent={accent} href={apiHref} />;
    case 'support':
      return <SupportSection section={section} accent={accent} />;
    default:
      return null;
  }
}
