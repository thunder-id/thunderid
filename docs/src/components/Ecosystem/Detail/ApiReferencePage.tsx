// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import {Box, Chip, Typography} from '@wso2/oxygen-ui';
import {Search} from '@wso2/oxygen-ui-icons-react';
import {ChangeEvent, JSX, useState} from 'react';
import {CodeCard, Pill} from './primitives';
import {DEFAULT_ACCENT, toneColour, useInk} from './theme';
import useEntryVersion from '../useEntryVersion';
import type {EcosystemApiSection, EcosystemEntry} from '@site/src/types/ecosystem';

/** Anchor for one export, stable enough to link to from the nav. */
function anchorOf(name: string): string {
  return name
    .replace(/[<>/()]/g, '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}

/**
 * The complete public surface of one package, on its own page.
 *
 * The entry's own page carries a two-pane explorer over a handful of exports;
 * this lists every one, with a filter and a nav grouped the way the package
 * groups them. Both read the same registry data, so they cannot disagree.
 */
export default function ApiReferencePage({entry}: {entry: EcosystemEntry}): JSX.Element | null {
  const ink = useInk();
  const version = useEntryVersion(entry);
  const [query, setQuery] = useState('');

  const section = entry.sections?.find((s): s is EcosystemApiSection => s.type === 'api');
  if (!section) return null;

  const accent = entry.accent ?? DEFAULT_ACCENT;
  const q = query.trim().toLowerCase();
  const matches = section.exports.filter((item) => !q || item.name.toLowerCase().includes(q));

  // Preserve the authored order of groups rather than sorting them.
  const groups = matches.reduce<{name: string; items: typeof matches}[]>((acc, item) => {
    const bucket = acc.find((g) => g.name === item.group);
    if (bucket) bucket.items.push(item);
    else acc.push({name: item.group, items: [item]});
    return acc;
  }, []);

  const eyebrowSx = {
    fontFamily: 'monospace',
    fontSize: '9px',
    letterSpacing: '0.13em',
    textTransform: 'uppercase' as const,
    color: ink(0.45, 0.45),
  };

  return (
    <Layout title={`API reference · ${entry.name}`} description={`The complete public surface of ${entry.package.name}.`}>
      <Box
        sx={{
          maxWidth: 1320,
          width: '100%',
          mx: 'auto',
          px: {xs: 2, sm: 4},
          pb: 10,
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: '236px minmax(0, 1fr)'},
          gap: {xs: 4, md: 6},
          alignItems: 'start',
        }}
      >
        <Box
          component="aside"
          sx={{
            position: {md: 'sticky'},
            top: {md: 80},
            alignSelf: 'start',
            pt: 4,
            maxHeight: {md: 'calc(100vh - 100px)'},
            overflowY: {md: 'auto'},
            minWidth: 0,
          }}
        >
          <Box sx={{position: 'relative', mb: 2.25}}>
            <Box
              sx={{
                position: 'absolute',
                left: 12,
                top: '50%',
                transform: 'translateY(-50%)',
                display: 'inline-flex',
                color: ink(0.35, 0.35),
                pointerEvents: 'none',
              }}
            >
              <Search size={14} />
            </Box>
            <Box
              component="input"
              type="text"
              value={query}
              aria-label="Filter exports"
              placeholder="Filter exports"
              onChange={(e: ChangeEvent<HTMLInputElement>) => setQuery(e.target.value)}
              sx={{
                width: '100%',
                height: 40,
                pl: '36px',
                pr: 1.5,
                fontSize: '13px',
                fontFamily: 'inherit',
                color: 'text.primary',
                bgcolor: ink(0.03, 0.04),
                border: '1px solid',
                borderColor: ink(0.1, 0.1),
                borderRadius: '10px',
                outline: 'none',
                '&:focus': {borderColor: `color-mix(in srgb, ${accent} 50%, transparent)`},
                '&::placeholder': {color: ink(0.35, 0.32)},
              }}
            />
          </Box>

          {groups.map((group) => (
            <Box key={group.name} sx={{mb: 2}}>
              <Box sx={{display: 'flex', alignItems: 'center', gap: 0.875, pb: 0.875, pl: '2px'}}>
                <Box
                  component="span"
                  sx={{
                    width: 5,
                    height: 5,
                    borderRadius: '50%',
                    flexShrink: 0,
                    bgcolor: toneColour(section.kindTones?.[group.items[0].kind], accent),
                  }}
                />
                <Typography component="span" sx={eyebrowSx}>
                  {group.name}
                </Typography>
                <Typography component="span" sx={{fontFamily: 'monospace', fontSize: '9.5px', color: ink(0.3, 0.3)}}>
                  {group.items.length}
                </Typography>
              </Box>
              <Box sx={{display: 'flex', flexDirection: 'column', borderLeft: '1px solid', borderColor: ink(0.08, 0.07)}}>
                {group.items.map((item) => (
                  <Box
                    key={item.name}
                    component="a"
                    href={`#${anchorOf(item.name)}`}
                    sx={{
                      fontFamily: 'monospace',
                      fontSize: '11.5px',
                      color: ink(0.6, 0.6),
                      textDecoration: 'none',
                      py: 0.625,
                      pl: 1.5,
                      ml: '-1px',
                      borderLeft: '2px solid transparent',
                      whiteSpace: 'nowrap',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      transition: 'all 0.15s',
                      '&:hover': {
                        color: 'text.primary',
                        borderLeftColor: accent,
                        bgcolor: `color-mix(in srgb, ${accent} 8%, transparent)`,
                      },
                    }}
                  >
                    {item.name}
                  </Box>
                ))}
              </Box>
            </Box>
          ))}

          {groups.length === 0 && (
            <Typography sx={{fontSize: '12.5px', lineHeight: 1.6, color: ink(0.5, 0.47), px: '2px'}}>
              No exports match that filter.
            </Typography>
          )}
        </Box>

        <Box component="main" sx={{minWidth: 0, pt: 4}}>
          <Box sx={{borderBottom: '1px solid', borderColor: ink(0.08, 0.07), pb: 3.25, mb: 4.25}}>
            <Box
              component="nav"
              aria-label="Breadcrumb"
              sx={{display: 'flex', alignItems: 'center', gap: 1, mb: 2.25, fontSize: '12.5px', color: ink(0.4, 0.35)}}
            >
              <Box component={Link} to="/sdks" sx={{color: 'inherit', textDecoration: 'none'}}>
                SDKs &amp; Tools
              </Box>
              <Box component="span" sx={{opacity: 0.5}}>
                /
              </Box>
              <Box component={Link} to={`/sdks/${entry.id}`} sx={{color: 'inherit', textDecoration: 'none'}}>
                {entry.name}
              </Box>
              <Box component="span" sx={{opacity: 0.5}}>
                /
              </Box>
              <Box component="span" sx={{color: ink(0.75, 0.8)}}>
                API reference
              </Box>
            </Box>

            <Box sx={{display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 2.5, flexWrap: 'wrap'}}>
              <Box sx={{minWidth: 0, flex: 1}}>
                <Typography
                  component="h1"
                  sx={{fontSize: '30px', fontWeight: 700, letterSpacing: '-0.035em', m: 0, mb: 1.25, color: 'text.primary'}}
                >
                  API reference
                </Typography>
                <Typography sx={{fontSize: '15px', lineHeight: 1.62, color: ink(0.6, 0.55), maxWidth: 620}}>
                  The public surface of{' '}
                  <Box component="code" sx={{fontFamily: 'monospace', fontSize: '13.5px', color: '#8bf9fa'}}>
                    {entry.package.name}
                  </Box>
                  , as documented in its reference pages.
                </Typography>
              </Box>
              {version && (
                <Chip
                  size="small"
                  variant="outlined"
                  label={version}
                  sx={{height: 22, color: '#4ade80', borderColor: 'rgba(74,222,128,0.5)', fontFamily: 'monospace'}}
                />
              )}
            </Box>
          </Box>

          {matches.map((item) => (
            <Box
              key={item.name}
              component="section"
              id={anchorOf(item.name)}
              sx={{borderTop: '1px solid', borderColor: ink(0.08, 0.07), pt: 4, mb: 4, scrollMarginTop: '84px'}}
            >
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1.25, flexWrap: 'wrap', mb: 1.25}}>
                <Pill label={item.kind} colour={toneColour(section.kindTones?.[item.kind], accent)} filled />
                <Typography
                  component="code"
                  sx={{fontFamily: 'monospace', fontSize: '19px', fontWeight: 500, letterSpacing: '-0.01em', color: 'text.primary'}}
                >
                  {item.name}
                </Typography>
              </Box>

              <Typography sx={{fontSize: '14px', lineHeight: 1.68, color: ink(0.6, 0.6), maxWidth: 640, mb: 2.5}}>
                {item.description}
              </Typography>

              {item.signature && (
                <>
                  <Typography component="div" sx={{...eyebrowSx, mb: 1}}>
                    Signature
                  </Typography>
                  <Box sx={{mb: 2.75}}>
                    <CodeCard code={{content: item.signature}} accent={accent} copyable={false} />
                  </Box>
                </>
              )}

              {item.rows && item.rows.length > 0 && (
                <>
                  <Typography component="div" sx={{...eyebrowSx, mb: 1}}>
                    {item.rowsLabel ?? 'Fields'}
                  </Typography>
                  <Box
                    sx={{
                      border: '1px solid',
                      borderColor: ink(0.08, 0.07),
                      borderRadius: '10px',
                      overflow: 'hidden',
                      bgcolor: ink(0.015, 0.02),
                      mb: 2.75,
                    }}
                  >
                    {item.rows.map((row) => (
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
                          alignItems: 'start',
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
                            {row.tag && <Pill label={row.tag} colour={row.tag === 'required' ? '#f59e0b' : ink(0.4, 0.4)} />}
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

              {item.example && (
                <>
                  <Typography component="div" sx={{...eyebrowSx, mb: 1}}>
                    Example
                  </Typography>
                  <CodeCard code={{content: item.example}} accent={accent} copyable={false} />
                </>
              )}
            </Box>
          ))}
        </Box>
      </Box>
    </Layout>
  );
}
