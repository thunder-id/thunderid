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
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import {Box, Button, Container, Stack, Typography, useTheme} from '@wso2/oxygen-ui';
import React, {JSX, useEffect, useState} from 'react';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import usePlatform from '../../hooks/usePlatform';
import {type ArchKey, type OsKey} from '../../utils/platform';
import ClaudeLogo from '../icons/ClaudeLogo';
import CliLogo from '../icons/CliLogo';
import CodexLogo from '../icons/CodexLogo';
import DockerLogo from '../icons/DockerLogo';
import GoLogo from '../icons/GoLogo';
import IOSLogo from '../icons/IOSLogo';
import LinuxLogo from '../icons/LinuxLogo';
import SkillsLogo from '../icons/SkillsLogo';
import WindowsLogo from '../icons/WindowsLogo';

const INSTALL_TABS = [
  {id: 'cli', label: 'npx', icon: CliLogo, command: 'npx thunderid', brandColor: null, enabled: true},
  {
    id: 'docker',
    label: 'Docker',
    icon: DockerLogo,
    command: 'docker compose -f oci://ghcr.io/thunder-id/thunderid-quick-start:latest up',
    brandColor: '#2560FF',
    enabled: true,
  },
  {
    id: 'claude',
    label: 'Claude',
    icon: ClaudeLogo,
    command: '/plugin marketplace add thunder-id/skills',
    brandColor: '#D97757',
    enabled: true,
  },
  {
    id: 'codex',
    label: 'Codex',
    icon: CodexLogo,
    command: 'codex plugin marketplace add thunder-id/skills',
    brandColor: '#3941FF',
    enabled: true,
  },
  {
    id: 'skills',
    label: 'Skills',
    icon: SkillsLogo,
    command: 'npx skills add thunder-id/skills',
    brandColor: null,
    enabled: true,
  },
];

interface DownloadAsset {
  arch: ArchKey;
  downloadUrl: string;
  os: OsKey;
  sizeLabel: string;
}

function parseDownloadAssets(assets: {name: string; downloadUrl: string; sizeLabel: string}[]): DownloadAsset[] {
  const result: DownloadAsset[] = [];
  for (const asset of assets) {
    const m = /^thunder(?:id)?-[0-9A-Za-z.+-]+-(macos|linux|win)-(arm64|x64)\.zip$/i.exec(asset.name);
    if (m) {
      result.push({
        arch: m[2] as ArchKey,
        downloadUrl: asset.downloadUrl,
        os: m[1] as OsKey,
        sizeLabel: asset.sizeLabel,
      });
    }
  }
  return result;
}

const OS_ICONS: Record<OsKey, JSX.Element> = {
  linux: <LinuxLogo size={20} />,
  macos: <IOSLogo size={18} />,
  win: <WindowsLogo size={20} />,
};

const OS_LABELS: Record<OsKey, string> = {
  linux: 'Linux',
  macos: 'macOS',
  win: 'Windows',
};

const ARCH_LABELS: Partial<Record<OsKey, Record<ArchKey, string>>> = {
  macos: {arm64: 'Apple Silicon', x64: 'Intel'},
};

export default function HeroSection(): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const {withBaseUrl} = useBaseUrlUtils();

  const [activeTab, setActiveTabRaw] = useState('cli');
  const setActiveTab = setActiveTabRaw as (v: string) => void;

  const [copied, setCopiedRaw] = useState(false);
  const setCopied = setCopiedRaw as (v: boolean) => void;
  const platform = usePlatform();

  const [downloadAssetsRaw, setDownloadAssetsRaw] = useState([] as DownloadAsset[]);
  const downloadAssets = downloadAssetsRaw;
  const setDownloadAssets = setDownloadAssetsRaw as (v: DownloadAsset[]) => void;
  useEffect(() => {
    fetch(withBaseUrl('/data/releases.json'))
      .then(
        (r) =>
          r.json() as Promise<{
            latestRelease: {tagName: string; assets: {name: string; downloadUrl: string; sizeLabel: string}[]};
          }>,
      )
      .then((data) => {
        setDownloadAssets(parseDownloadAssets(data.latestRelease?.assets ?? []));
      })
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      .catch(() => {});
  }, [withBaseUrl, setDownloadAssets]);

  const activeCommand = INSTALL_TABS.find((t) => t.id === activeTab)?.command ?? '';

  const cmdSpaceIdx = activeCommand.indexOf(' ');
  const cmdFirst = cmdSpaceIdx !== -1 ? activeCommand.slice(0, cmdSpaceIdx) : activeCommand;
  const cmdRest = cmdSpaceIdx !== -1 ? activeCommand.slice(cmdSpaceIdx) : '';

  const handleCopy = (): void => {
    void navigator.clipboard.writeText(activeCommand).then(() => {
      setCopied(true);
      setTimeout(() => {
        setCopied(false);
      }, 1800);
    });
  };

  const primaryAsset: DownloadAsset | null = (() => {
    if (!platform?.os || downloadAssets.length === 0) return null;
    const preferred = platform.arch ?? 'arm64';
    const typed = downloadAssets;
    return (
      typed.find((a: DownloadAsset) => a.os === platform.os && a.arch === preferred) ??
      typed.find((a: DownloadAsset) => a.os === platform.os) ??
      null
    );
  })();

  const alternateAsset: DownloadAsset | null = (() => {
    if (!primaryAsset || !platform?.os) return null;
    const alt: ArchKey = primaryAsset.arch === 'arm64' ? 'x64' : 'arm64';
    return downloadAssets.find((a: DownloadAsset) => a.os === platform.os && a.arch === alt) ?? null;
  })();

  const dimColor = isLight ? 'rgba(0,0,0,0.38)' : 'rgba(255,255,255,0.35)';

  return (
    <Box
      sx={{
        '@keyframes fadeInUp': {
          from: {opacity: 0, transform: 'translateY(24px)'},
          to: {opacity: 1, transform: 'translateY(0)'},
        },
        minHeight: 'calc(100vh - var(--ifm-navbar-height))',
        display: 'flex',
        alignItems: 'center',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}, position: 'relative', zIndex: 1, width: '100%'}}>
        <Box
          sx={{
            '@keyframes streak': {
              '0%': {transform: 'translateX(-120%) skewX(-20deg)', opacity: 0},
              '20%': {opacity: 0.18},
              '80%': {opacity: 0.18},
              '100%': {transform: 'translateX(260%) skewX(-20deg)', opacity: 0},
            },
            display: 'flex',
            flexDirection: {xs: 'column', lg: 'row'},
            alignItems: {xs: 'center', lg: 'center'},
            gap: {xs: 6, lg: 8},
            py: {xs: 8, lg: 10},
          }}
        >
          {/* ── Left column ── */}
          <Box
            sx={{
              flex: '1 1 0',
              display: 'flex',
              flexDirection: 'column',
              alignItems: {xs: 'center', lg: 'flex-start'},
              textAlign: {xs: 'center', lg: 'left'},
            }}
          >
            <Typography
              variant="h1"
              sx={{
                mb: 2.5,
                fontSize: {xs: '3rem', sm: '4rem', md: '5rem'},
                fontWeight: 700,
                lineHeight: 1.05,
                color: 'text.primary',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.1s both',
              }}
            >
              <Box
                component="span"
                sx={{
                  background: `linear-gradient(90deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                  backgroundClip: 'text',
                }}
              >
                Auth
              </Box>{' '}
              for Modern Apps and Agents
            </Typography>

            <Typography
              variant="body1"
              sx={{
                maxWidth: '480px',
                mb: 5,
                fontSize: {xs: '1.05rem', sm: '1.15rem'},
                lineHeight: 1.75,
                color: 'text.secondary',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.2s both',
              }}
            >
              Authentication, authorization, and identity for humans, AI agents, and workloads.
            </Typography>

            <Stack
              direction={{xs: 'column', sm: 'row'}}
              spacing={2}
              alignItems="center"
              sx={{animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.3s both', mb: 4}}
            >
              <Button
                component={Link}
                href="/docs/next"
                variant="contained"
                color="primary"
                size="large"
                sx={{
                  px: 4,
                  py: 1.5,
                  fontWeight: 600,
                  textTransform: 'none',
                  fontSize: '1rem',
                  borderRadius: '28px',
                  background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                  transition: 'transform 0.2s ease, box-shadow 0.2s ease',
                  '&:hover': {
                    background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                    transform: 'translateY(-2px)',
                    boxShadow: `0 6px 24px rgba(${theme.vars?.palette.primary.main} / 0.35)`,
                  },
                  '&:active': {transform: 'translateY(0)'},
                }}
              >
                Get Started
              </Button>
              <Button
                component={Link}
                href="/docs/next/guides/getting-started/get-thunderid"
                variant="text"
                size="large"
                sx={{
                  px: 3,
                  py: 1.5,
                  textTransform: 'none',
                  fontSize: '1rem',
                  color: 'text.secondary',
                  borderRadius: '28px',
                  '&:hover': {bgcolor: 'transparent', color: 'text.primary'},
                }}
              >
                Learn More →
              </Button>
            </Stack>

            <Box
              sx={{
                display: 'flex',
                flexWrap: 'wrap',
                gap: {xs: 1.5, sm: 0},
                alignItems: 'center',
                justifyContent: {xs: 'center', lg: 'flex-start'},
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.4s both',
              }}
            >
              {(
                [
                  {
                    key: 'apache',
                    icon: null,
                    parts: [
                      {text: 'Apache 2.0', bold: true},
                      {text: 'license', bold: false},
                    ],
                  },
                  {
                    key: 'go',
                    icon: <GoLogo size={18} />,
                    parts: [
                      {text: 'built with', bold: false},
                      {text: 'Go', bold: true},
                    ],
                  },
                  {
                    key: 'speed',
                    icon: null,
                    parts: [
                      {text: '<50ms', bold: true},
                      {text: 'startup', bold: false},
                    ],
                  },
                ] as {key: string; icon: JSX.Element | null; parts: {text: string; bold: boolean}[]}[]
              ).map(({key, icon, parts}, i) => (
                <React.Fragment key={key}>
                  {i > 0 && (
                    <Box
                      component="span"
                      sx={{
                        mx: 2,
                        display: {xs: 'none', sm: 'inline'},
                        color: isLight ? 'rgba(0,0,0,0.15)' : 'rgba(255,255,255,0.12)',
                      }}
                    >
                      ·
                    </Box>
                  )}
                  <Box sx={{display: 'inline-flex', alignItems: 'center', gap: 0.5}}>
                    {icon && (
                      <Box sx={{display: 'flex', alignItems: 'center', filter: 'grayscale(0.4)', opacity: 0.65}}>
                        {icon}
                      </Box>
                    )}
                    {parts.map(({text, bold}) => (
                      <Typography
                        key={text}
                        component="span"
                        sx={{
                          fontSize: '0.8rem',
                          ...(bold
                            ? {
                                fontWeight: 600,
                                fontFamily: 'monospace',
                                color: isLight ? 'rgba(0,0,0,0.55)' : 'rgba(255,255,255,0.6)',
                              }
                            : {color: isLight ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.3)'}),
                        }}
                      >
                        {text}
                      </Typography>
                    ))}
                  </Box>
                </React.Fragment>
              ))}
            </Box>
          </Box>

          {/* ── Right column ── */}
          <Box
            sx={{
              flex: '1 1 0',
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
              width: '100%',
              maxWidth: {xs: 560, lg: 'none'},
              animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.25s both',
            }}
          >
            {/* Section label */}
            <Box sx={{mb: 2.25}}>
              <Typography
                sx={{
                  fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                  fontSize: '0.8rem',
                  textTransform: 'uppercase',
                  letterSpacing: '0.14em',
                  color: isLight ? 'rgba(0,0,0,0.28)' : 'rgba(255,255,255,0.28)',
                  mb: 0.875,
                }}
              >
                Get ThunderID where you work
              </Typography>
              <Box
                sx={{
                  width: 32,
                  height: 2,
                  borderRadius: '1px',
                  background: `linear-gradient(90deg, ${theme.vars?.palette.primary.main}, rgba(54,136,255,0.2))`,
                }}
              />
            </Box>

            {/* Tab row */}
            <Box sx={{display: 'flex', gap: 1, mb: 2, flexWrap: 'wrap'}}>
              {INSTALL_TABS.filter((tab) => tab.enabled).map((tab) => {
                const isActive = activeTab === tab.id;
                const ac = isActive && tab.brandColor ? tab.brandColor : null;
                const activeColor = ac ?? theme.vars?.palette.primary.main;
                return (
                  <Box
                    key={tab.id}
                    onClick={() => {
                      setActiveTab(tab.id);
                    }}
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 0.75,
                      px: {xs: 1.5, sm: 2},
                      py: {xs: 0.75, sm: 1},
                      borderRadius: '8px',
                      border: '1px solid',
                      borderColor: isActive
                        ? ac
                          ? `${ac}80`
                          : `rgba(${theme.vars?.palette.primary.main} / 0.5)`
                        : isLight
                          ? 'rgba(0,0,0,0.12)'
                          : 'rgba(255,255,255,0.1)',
                      bgcolor: isActive
                        ? ac
                          ? `${ac}1A`
                          : `rgba(${theme.vars?.palette.primary.main} / 0.1)`
                        : 'transparent',
                      color: isActive ? activeColor : isLight ? 'rgba(0,0,0,0.55)' : 'rgba(255,255,255,0.5)',
                      fontFamily: 'monospace',
                      fontSize: {xs: '0.78rem', sm: '0.85rem'},
                      fontWeight: isActive ? 600 : 400,
                      cursor: 'pointer',
                      transition: 'all 0.15s ease',
                      userSelect: 'none',
                      '&:hover': {
                        borderColor: isActive
                          ? ac
                            ? `${ac}99`
                            : `rgba(${theme.vars?.palette.primary.main} / 0.6)`
                          : isLight
                            ? 'rgba(0,0,0,0.25)'
                            : 'rgba(255,255,255,0.22)',
                        color: isActive ? activeColor : isLight ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.8)',
                      },
                    }}
                  >
                    <Box
                      component="span"
                      sx={{
                        display: 'flex',
                        filter: isActive ? 'none' : 'grayscale(1)',
                        opacity: isActive ? 1 : 0.5,
                        transition: 'filter 0.15s ease, opacity 0.15s ease',
                      }}
                    >
                      <tab.icon />
                    </Box>
                    {tab.label}
                  </Box>
                );
              })}
            </Box>

            {/* Command display */}
            <Box
              sx={{
                '@keyframes glowPulse': {
                  '0%, 100%': {
                    borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
                    boxShadow: 'none',
                  },
                  '50%': {
                    borderColor: theme.vars?.palette.primary.main,
                    boxShadow: `0 0 22px 2px ${theme.vars?.palette.primary.main}`,
                  },
                },
                display: 'flex',
                alignItems: 'center',
                gap: 2,
                px: {xs: 2.5, sm: 3.5},
                py: {xs: 2, sm: 2.5},
                borderRadius: '14px',
                border: '1px solid',
                borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
                bgcolor: isLight ? 'rgba(0,0,0,0.03)' : 'rgba(255,255,255,0.04)',
                backdropFilter: 'blur(8px)',
                animation: 'glowPulse 3s ease-in-out 0.8s infinite',
              }}
            >
              <Typography
                component="code"
                sx={{
                  flex: 1,
                  minWidth: 0,
                  fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                  fontSize: {xs: '0.95rem', sm: '1.1rem'},
                  fontWeight: 500,
                  color: isLight ? 'rgba(0,0,0,0.75)' : 'rgba(255,255,255,0.85)',
                  textAlign: 'left',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  letterSpacing: '-0.01em',
                  background: 'none',
                  backgroundColor: 'transparent',
                  border: 'none',
                  padding: 0,
                  borderRadius: 0,
                }}
              >
                <Box
                  component="span"
                  sx={{color: isLight ? 'rgba(0,0,0,0.28)' : 'rgba(255,255,255,0.22)', mr: 1.5, fontWeight: 400}}
                >
                  $
                </Box>
                <Box component="span" sx={{color: theme.vars?.palette.success.main, fontWeight: 600}}>
                  {cmdFirst}
                </Box>
                {cmdRest}
              </Typography>

              <Box
                onClick={handleCopy}
                title={copied ? 'Copied!' : 'Copy'}
                sx={{
                  flexShrink: 0,
                  position: 'relative',
                  overflow: 'hidden',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 0.75,
                  px: 1.75,
                  py: 0.875,
                  borderRadius: '8px',
                  border: '1px solid',
                  borderColor: copied
                    ? 'rgba(74,222,128,0.4)'
                    : isLight
                      ? 'rgba(0,0,0,0.12)'
                      : 'rgba(255,255,255,0.12)',
                  bgcolor: copied ? 'rgba(74,222,128,0.08)' : isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.05)',
                  color: copied ? '#4ade80' : isLight ? 'rgba(0,0,0,0.5)' : 'rgba(255,255,255,0.45)',
                  cursor: 'pointer',
                  transition: 'all 0.18s ease',
                  '&:hover': {
                    borderColor: copied
                      ? 'rgba(74,222,128,0.5)'
                      : isLight
                        ? 'rgba(0,0,0,0.25)'
                        : 'rgba(255,255,255,0.28)',
                    color: copied ? '#4ade80' : isLight ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.85)',
                  },
                  '&::after': {
                    content: '""',
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '40%',
                    height: '100%',
                    background: `linear-gradient(90deg, transparent 0%, ${isLight ? 'rgba(255,255,255,0.55)' : 'rgba(255,255,255,0.12)'} 50%, transparent 100%)`,
                    animation: 'streak 3.5s ease-in-out 1.2s infinite',
                    pointerEvents: 'none',
                  },
                }}
              >
                {copied ? (
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                ) : (
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                    <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                  </svg>
                )}
                <Typography component="span" sx={{fontSize: '0.78rem', fontWeight: 500, lineHeight: 1}}>
                  {copied ? 'Copied' : 'Copy'}
                </Typography>
              </Box>
            </Box>

            {/* ── or divider ── */}
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, mt: 2}}>
              <Box
                sx={{flex: 1, height: '1px', bgcolor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)'}}
              />
              <Box
                component="span"
                sx={{
                  fontFamily: '"JetBrains Mono", "Fira Code", monospace',
                  fontSize: '0.8rem',
                  color: isLight ? 'rgba(0,0,0,0.25)' : 'rgba(255,255,255,0.25)',
                  letterSpacing: '0.08em',
                }}
              >
                or
              </Box>
              <Box
                sx={{flex: 1, height: '1px', bgcolor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)'}}
              />
            </Box>

            {/* ── Download row ── */}
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, flexWrap: 'wrap', mt: 1.75}}>
              {primaryAsset ? (
                <>
                  <Box
                    component="a"
                    href={primaryAsset.downloadUrl}
                    sx={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 0.75,
                      textDecoration: 'none',
                      color: isLight ? 'rgba(0,0,0,0.5)' : 'rgba(255,255,255,0.45)',
                      fontSize: '0.82rem',
                      transition: 'color 0.15s ease',
                      '&:hover': {color: isLight ? 'rgba(0,0,0,0.8)' : 'rgba(255,255,255,0.85)'},
                    }}
                  >
                    <Box sx={{display: 'flex', alignItems: 'center'}}>{OS_ICONS[primaryAsset.os]}</Box>
                    <span>
                      Download for {OS_LABELS[primaryAsset.os]}
                      {ARCH_LABELS[primaryAsset.os]?.[primaryAsset.arch]
                        ? ` (${ARCH_LABELS[primaryAsset.os]?.[primaryAsset.arch] ?? ''})`
                        : ''}
                    </span>
                  </Box>
                  {alternateAsset && (
                    <>
                      <Box component="span" sx={{color: isLight ? 'rgba(0,0,0,0.15)' : 'rgba(255,255,255,0.15)'}}>
                        ·
                      </Box>
                      <Typography
                        component="a"
                        href={alternateAsset.downloadUrl}
                        sx={{
                          fontSize: '0.82rem',
                          color: dimColor,
                          textDecoration: 'none',
                          '&:hover': {color: theme.vars?.palette.primary.main, textDecoration: 'underline'},
                          transition: 'color 0.15s ease',
                        }}
                      >
                        {ARCH_LABELS[alternateAsset.os]?.[alternateAsset.arch] ?? alternateAsset.arch}
                      </Typography>
                    </>
                  )}
                  <Box component="span" sx={{color: isLight ? 'rgba(0,0,0,0.15)' : 'rgba(255,255,255,0.15)'}}>
                    ·
                  </Box>
                </>
              ) : null}
              <Typography
                component={Link}
                href="/docs/next/guides/getting-started/get-thunderid"
                sx={{
                  fontSize: '0.82rem',
                  color: dimColor,
                  textDecoration: 'none',
                  '&:hover': {color: theme.vars?.palette.primary.main, textDecoration: 'underline'},
                  transition: 'color 0.15s ease',
                }}
              >
                Other platforms →
              </Typography>
            </Box>
          </Box>
        </Box>
      </Container>
    </Box>
  );
}
