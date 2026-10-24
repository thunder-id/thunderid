// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {Box, Button, ButtonBase, Container, Stack, Typography, useTheme} from '@wso2/oxygen-ui';
import {ArrowRight, Check, Star} from '@wso2/oxygen-ui-icons-react';
import {JSX, useEffect, useState} from 'react';
import {DISPLAY_FONT, gradientTextSx, HOME_MAX_WIDTH} from './homeTheme';
import StandardsMarquee from './StandardsMarquee';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import usePlatform from '../../hooks/usePlatform';
import {type ArchKey, type OsKey} from '../../utils/platform';
import ClaudeLogo from '../icons/ClaudeLogo';
import CliLogo from '../icons/CliLogo';
import CodexLogo from '../icons/CodexLogo';
import DockerLogo from '../icons/DockerLogo';
import IOSLogo from '../icons/IOSLogo';
import LinuxLogo from '../icons/LinuxLogo';
import SkillsLogo from '../icons/SkillsLogo';
import WindowsLogo from '../icons/WindowsLogo';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

const INSTALL_TABS = [
  {id: 'cli', label: 'npx', icon: CliLogo, command: 'npx thunderid', enabled: true},
  {
    id: 'docker',
    label: 'Docker',
    icon: DockerLogo,
    command: 'docker compose -f oci://ghcr.io/thunder-id/thunderid-quick-start:latest up',
    enabled: true,
  },
  {
    id: 'claude',
    label: 'Claude',
    icon: ClaudeLogo,
    command: '/plugin marketplace add thunder-id/skills',
    enabled: true,
  },
  {
    id: 'codex',
    label: 'Codex',
    icon: CodexLogo,
    command: 'codex plugin marketplace add thunder-id/skills',
    enabled: true,
  },
  {
    id: 'skills',
    label: 'Skills',
    icon: SkillsLogo,
    command: 'npx skills add thunder-id/skills',
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
  const docsUrl = useDocsUrl();
  const {siteConfig} = useDocusaurusContext();
  const product = siteConfig.customFields?.product as DocusaurusProductConfig;
  const productName = product.project.name;
  const githubUrl = product.project.source.github.url;

  const [activeTab, setActiveTabRaw] = useState('cli');
  const setActiveTab = setActiveTabRaw as (v: string) => void;

  const [copied, setCopiedRaw] = useState(false);
  const setCopied = setCopiedRaw as (v: boolean) => void;
  const platform = usePlatform();

  const [releaseVersion, setReleaseVersion] = useState<string | null>(null);
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
        // Announce the major.minor line (e.g. "v1.0.1" → "1.0").
        const m = /^v?(\d+)\.(\d+)/.exec(data.latestRelease?.tagName ?? '');
        setReleaseVersion(m ? `${m[1]}.${m[2]}` : null);
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

  const lineStrong = isLight ? 'rgba(15,40,80,0.16)' : 'rgba(160,200,255,0.18)';
  const muted2 = isLight ? 'rgba(15,30,60,0.48)' : '#5b6b85';

  return (
    <Box
      sx={{
        '@keyframes fadeInUp': {
          from: {opacity: 0, transform: 'translateY(24px)'},
          to: {opacity: 1, transform: 'translateY(0)'},
        },
        minHeight: 'calc(100svh - var(--ifm-navbar-height))',
        display: 'flex',
        flexDirection: 'column',
        position: 'relative',
        overflow: 'hidden',
      }}
    >
      <Container
        maxWidth={false}
        sx={{maxWidth: HOME_MAX_WIDTH, px: {xs: 2, sm: 4}, position: 'relative', zIndex: 1, width: '100%', flex: 1, display: 'flex', alignItems: 'center'}}
      >
        <Box
          sx={{
            display: 'flex',
            flexDirection: {xs: 'column', lg: 'row'},
            alignItems: {xs: 'center', lg: 'center'},
            width: '100%',
            gap: {xs: 6, lg: 6},
            pt: {xs: 5, lg: 8},
            pb: {xs: 5, lg: 6},
          }}
        >
          {/* ── Left column ── */}
          <Box
            sx={{
              flex: '1.05 1 0',
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
              alignItems: {xs: 'center', lg: 'flex-start'},
              textAlign: {xs: 'center', lg: 'left'},
            }}
          >
            {/* Release announcement */}
            <Box
              component={Link}
              to="/releases"
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 1.25,
                mb: {xs: 2.5, md: 4},
                p: '6px 6px 6px 14px',
                maxWidth: '100%',
                borderRadius: '999px',
                fontSize: {xs: '0.8125rem', sm: '0.875rem'},
                color: 'text.primary',
                textDecoration: 'none',
                bgcolor: isLight ? 'rgba(255,255,255,0.7)' : 'rgba(13,17,23,0.6)',
                border: '1px solid',
                borderColor: isLight ? 'rgba(15,40,80,0.14)' : '#1f3350',
                backdropFilter: 'blur(10px)',
                transition: 'border-color 0.2s',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0s both',
                '&:hover': {borderColor: theme.vars?.palette.primary.main, color: 'text.primary', textDecoration: 'none'},
              }}
            >
              <Box component="span" sx={{whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis'}}>
                {releaseVersion ? `${productName} ${releaseVersion} is here` : `What's new in ${productName}`}
              </Box>
              <Box
                component="span"
                sx={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: 0.75,
                  flex: 'none',
                  px: 1.25,
                  py: 0.5,
                  borderRadius: '999px',
                  bgcolor: 'rgba(54,136,255,0.15)',
                  color: 'primary.main',
                  fontWeight: 600,
                  fontSize: '0.8125rem',
                }}
              >
                <Box component="span" sx={{display: {xs: 'none', sm: 'inline'}}}>
                  Read the release notes
                </Box>
                <Box component="span" sx={{display: {xs: 'inline', sm: 'none'}}}>
                  Release notes
                </Box>
                <ArrowRight size={14} strokeWidth={2} />
              </Box>
            </Box>

            <Typography
              variant="h1"
              sx={{
                mb: 0,
                fontFamily: DISPLAY_FONT,
                fontSize: {xs: '2.25rem', sm: '3rem', lg: '3.75rem'},
                fontWeight: 700,
                lineHeight: 1.05,
                letterSpacing: '-0.035em',
                textWrap: 'balance',
                color: 'text.primary',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.1s both',
                '& .hl': {color: 'primary.main'},
              }}
            >
              <Box component="span" sx={gradientTextSx}>
                Open source
              </Box>{' '}
              identity for <span className="hl">humans</span>, <span className="hl">AI&nbsp;agents</span>, and&nbsp;
              <span className="hl">machines</span>
            </Typography>

            <Typography
              variant="body1"
              sx={{
                mt: 2.75,
                maxWidth: 600,
                fontSize: {xs: '1.0625rem', sm: '1.3125rem'},
                lineHeight: 1.55,
                color: 'text.secondary',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.2s both',
              }}
            >
              Authentication, authorization, and identity management, built for the agentic era. Native agent identity,
              post-quantum safety, and ready for digital wallets. A lightweight, high-performance Go runtime.
            </Typography>

            <Stack
              direction={{xs: 'column', sm: 'row'}}
              spacing={1.5}
              alignItems="center"
              sx={{mt: 4.5, width: {xs: '100%', sm: 'auto'}, animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.3s both'}}
            >
              <Button
                component={Link}
                href={docsUrl('/docs/next')}
                variant="contained"
                color="primary"
                size="large"
                endIcon={<ArrowRight size={17} strokeWidth={2} />}
                sx={{
                  height: 52,
                  px: 3,
                  width: {xs: '100%', sm: 'auto'},
                  fontWeight: 600,
                  textTransform: 'none',
                  fontSize: '1rem',
                  borderRadius: '8px',
                  boxShadow: 'inset 0 1px 0 rgba(255,255,255,0.12)',
                  transition: 'transform 0.18s ease, background-color 0.2s ease',
                  '&:hover': {transform: 'translateY(-2px)', color: 'common.white'},
                }}
              >
                Get started
              </Button>
              <Button
                component="a"
                href={githubUrl}
                target="_blank"
                rel="noopener noreferrer"
                variant="outlined"
                size="large"
                startIcon={<Star size={17} />}
                sx={{
                  height: 52,
                  px: 3,
                  width: {xs: '100%', sm: 'auto'},
                  fontWeight: 600,
                  textTransform: 'none',
                  fontSize: '1rem',
                  borderRadius: '8px',
                  color: 'text.primary',
                  borderColor: isLight ? 'rgba(15,40,80,0.16)' : 'rgba(160,200,255,0.18)',
                  bgcolor: isLight ? 'rgba(255,255,255,0.6)' : 'rgba(4,10,22,0.55)',
                  backdropFilter: 'blur(10px)',
                  transition: 'transform 0.18s ease, border-color 0.2s ease',
                  '&:hover': {
                    transform: 'translateY(-2px)',
                    color: 'text.primary',
                    borderColor: isLight ? 'rgba(15,40,80,0.3)' : '#5b6b85',
                    bgcolor: isLight ? 'rgba(255,255,255,0.9)' : 'rgba(22,27,34,0.8)',
                  },
                }}
              >
                Star on GitHub
              </Button>
            </Stack>

            <Box
              sx={{
                display: 'inline-flex',
                flexWrap: 'wrap',
                justifyContent: 'center',
                gap: {xs: 1, sm: 2.25},
                mt: 2.75,
                px: 2,
                py: 1,
                borderRadius: {xs: '16px', sm: '999px'},
                fontSize: '0.84375rem',
                color: 'text.secondary',
                bgcolor: isLight ? 'rgba(255,255,255,0.6)' : 'rgba(4,10,22,0.55)',
                border: '1px solid',
                borderColor: isLight ? 'rgba(15,40,80,0.09)' : 'rgba(160,200,255,0.10)',
                backdropFilter: 'blur(10px)',
                animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.4s both',
              }}
            >
              {['Apache 2.0 license', 'Built with Go', '<50ms startup'].map((label) => (
                <Box key={label} component="span" sx={{display: 'inline-flex', alignItems: 'center', gap: 0.75}}>
                  <Check size={14} strokeWidth={2.4} color={isLight ? '#0092c2' : '#00D5FF'} />
                  {label}
                </Box>
              ))}
            </Box>
          </Box>
          {/* ── Right column ── */}
          <Box
            sx={{
              flex: '0.95 1 0',
              minWidth: 0,
              display: 'flex',
              flexDirection: 'column',
              width: '100%',
              maxWidth: {xs: 560, lg: 'none'},
              animation: 'fadeInUp 0.7s cubic-bezier(0.16,1,0.3,1) 0.25s both',
            }}
          >
            {/* Section label */}
            <Box sx={{mb: 2.25, textAlign: 'left'}}>
              <Typography
                component="h2"
                sx={{
                  m: 0,
                  fontFamily: DISPLAY_FONT,
                  fontSize: '1rem',
                  fontWeight: 600,
                  lineHeight: 1.3,
                  letterSpacing: '-0.01em',
                  color: 'text.primary',
                }}
              >
                Get {productName} where you work
              </Typography>
            </Box>

            {/* Tab row */}
            <Box role="tablist" aria-label={`Ways to get ${productName}`} sx={{display: 'flex', gap: 1.25, flexWrap: 'wrap'}}>
              {INSTALL_TABS.filter((tab) => tab.enabled).map((tab) => {
                const isActive = activeTab === tab.id;
                return (
                  <ButtonBase
                    key={tab.id}
                    role="tab"
                    aria-selected={isActive}
                    onClick={() => {
                      setActiveTab(tab.id);
                    }}
                    sx={{
                      gap: 1,
                      px: {xs: 1.5, sm: 2},
                      py: {xs: 1, sm: 1.375},
                      borderRadius: '10px',
                      border: '1px solid',
                      borderColor: isActive ? 'primary.main' : lineStrong,
                      bgcolor: isActive ? 'rgba(54,136,255,0.1)' : 'transparent',
                      color: isActive ? 'text.primary' : 'text.secondary',
                      fontFamily: 'var(--ifm-font-family-monospace)',
                      fontSize: {xs: '0.875rem', sm: '0.9375rem'},
                      fontWeight: 400,
                      transition: 'color 0.2s, border-color 0.2s, background-color 0.2s',
                      '&:hover': {
                        borderColor: isActive ? 'primary.main' : muted2,
                        color: 'text.primary',
                      },
                      '& svg': {width: 15, height: 15},
                    }}
                  >
                    <Box component="span" sx={{display: 'flex'}}>
                      <tab.icon />
                    </Box>
                    {tab.label}
                  </ButtonBase>
                );
              })}
            </Box>

            {/* Command display */}
            <Box
              role="tabpanel"
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1.5,
                mt: 2,
                minHeight: {xs: 72, sm: 88},
                py: 1.5,
                pl: {xs: 2, sm: 2.75},
                pr: 1.75,
                borderRadius: '14px',
                border: '1px solid',
                borderColor: isLight ? 'rgba(15,40,80,0.14)' : '#1f3350',
                bgcolor: isLight ? '#f5f8fc' : '#0a1120',
              }}
            >
              <Typography
                component="code"
                sx={{
                  flex: 1,
                  minWidth: 0,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1.5,
                  fontFamily: 'var(--ifm-font-family-monospace)',
                  fontSize: {xs: '0.9375rem', sm: '1.1875rem'},
                  color: 'text.primary',
                  textAlign: 'left',
                  whiteSpace: 'nowrap',
                  overflowX: 'auto',
                  scrollbarWidth: 'none',
                  '&::-webkit-scrollbar': {display: 'none'},
                  background: 'none',
                  backgroundColor: 'transparent',
                  border: 'none',
                  padding: 0,
                  borderRadius: 0,
                }}
              >
                <Box component="span" sx={{color: muted2}}>
                  {activeTab === 'claude' ? '>' : '$'}
                </Box>
                <span>
                  <Box component="b" sx={{fontWeight: 600, color: isLight ? '#1a7f37' : '#8ff0a4'}}>
                    {cmdFirst}
                  </Box>
                  {cmdRest}
                </span>
              </Typography>

              <ButtonBase
                onClick={handleCopy}
                aria-label={copied ? 'Copied' : 'Copy command'}
                sx={{
                  flexShrink: 0,
                  height: 40,
                  px: 2,
                  borderRadius: '8px',
                  border: '1px solid',
                  borderColor: lineStrong,
                  bgcolor: isLight ? 'rgba(15,40,80,0.03)' : 'rgba(200,225,255,0.04)',
                  fontFamily: 'var(--ifm-font-family-monospace)',
                  fontSize: '0.875rem',
                  color: copied ? '#3fb950' : 'text.secondary',
                  transition: 'color 0.2s, border-color 0.2s',
                  '&:hover': {color: copied ? '#3fb950' : 'text.primary', borderColor: muted2},
                }}
              >
                {copied ? 'Copied' : 'Copy'}
              </ButtonBase>
            </Box>

            {/* ── or divider ── */}
            <Box
              aria-hidden
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1.5,
                mt: 3,
                mb: 2.25,
                fontFamily: 'var(--ifm-font-family-monospace)',
                fontSize: '0.8125rem',
                letterSpacing: '0.08em',
                color: muted2,
              }}
            >
              <Box sx={{flex: 1, height: '1px', bgcolor: lineStrong}} />
              or
              <Box sx={{flex: 1, height: '1px', bgcolor: lineStrong}} />
            </Box>

            {/* ── Download row ── */}
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px 14px',
                flexWrap: 'wrap',
                fontSize: {xs: '0.9375rem', sm: '1.03rem'},
                '& .dl-sep': {color: muted2},
                '& .dl-other': {
                  color: 'text.secondary',
                  fontWeight: 500,
                  textDecoration: 'none',
                  transition: 'color 0.2s',
                  '&:hover': {color: 'text.primary', textDecoration: 'none'},
                },
              }}
            >
              {primaryAsset ? (
                <>
                  <Box
                    component="a"
                    href={primaryAsset.downloadUrl}
                    sx={{
                      display: 'inline-flex',
                      alignItems: 'center',
                      gap: 1.25,
                      fontWeight: 500,
                      color: 'text.primary',
                      textDecoration: 'none',
                      transition: 'color 0.2s',
                      '&:hover': {color: isLight ? '#2a74e0' : '#7cb4ff', textDecoration: 'none'},
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
                      <span className="dl-sep" aria-hidden>
                        ·
                      </span>
                      <a className="dl-other" href={alternateAsset.downloadUrl}>
                        {ARCH_LABELS[alternateAsset.os]?.[alternateAsset.arch] ?? alternateAsset.arch}
                      </a>
                    </>
                  )}
                  <span className="dl-sep" aria-hidden>
                    ·
                  </span>
                </>
              ) : null}
              <Link className="dl-other" to={docsUrl('/docs/next/getting-started/get-thunderid')}>
                Other platforms →
              </Link>
            </Box>
          </Box>
        </Box>
      </Container>
      <StandardsMarquee />
    </Box>
  );
}
