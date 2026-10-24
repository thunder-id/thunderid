// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {AndroidLogo, FlutterLogo} from '@thunderid/components';
import {Box, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {DISPLAY_FONT, useHomeTokens} from './homeTheme';
import {TextLink} from './shared';
import ExpressLogo from '../icons/ExpressLogo';
import IOSLogo from '../icons/IOSLogo';
import JavaScriptLogo from '../icons/JavaScriptLogo';
import NextLogo from '../icons/NextLogo';
import NodeLogo from '../icons/NodeLogo';
import NuxtLogo from '../icons/NuxtLogo';
import ReactLogo from '../icons/ReactLogo';
import VueLogo from '../icons/VueLogo';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

interface Sdk {
  name: string;
  icon: (props: {size?: number}) => JSX.Element;
  href: string;
  /** Shown in the compact strip on the home page. */
  featured?: boolean;
}

const SDKS: Sdk[] = [
  {name: 'React', icon: ReactLogo, href: '/docs/next/getting-started/connect-your-application/react', featured: true},
  {name: 'Next.js', icon: NextLogo, href: '/docs/next/getting-started/connect-your-application/nextjs', featured: true},
  {name: 'Express', icon: ExpressLogo, href: '/docs/next/getting-started/connect-your-application/express'},
  {name: 'Vue', icon: VueLogo, href: '/docs/next/getting-started/connect-your-application/vue'},
  {name: 'Nuxt', icon: NuxtLogo, href: '/docs/next/getting-started/connect-your-application/nuxt'},
  {name: 'Node.js', icon: NodeLogo, href: '/docs/next/getting-started/connect-your-application/node', featured: true},
  {
    name: 'Vanilla JavaScript',
    icon: JavaScriptLogo,
    href: '/docs/next/getting-started/connect-your-application/browser',
  },
  {name: 'iOS', icon: IOSLogo, href: '/docs/next/getting-started/connect-your-application/ios', featured: true},
  {name: 'Android', icon: AndroidLogo, href: '/docs/next/getting-started/connect-your-application/android'},
  {
    name: 'Flutter',
    icon: FlutterLogo,
    href: '/docs/next/getting-started/connect-your-application/flutter',
    featured: true,
  },
];

/**
 * Body of the "SDKs" card in the features bento: copy on the left, a strip of
 * featured SDK tiles on the right.
 */
export default function SDKShowcase(): JSX.Element {
  const t = useHomeTokens();
  const docsUrl = useDocsUrl();
  const featured = SDKS.filter((s) => s.featured);

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: {xs: '1fr', md: 'minmax(220px, 0.7fr) 2.3fr'},
        alignItems: 'center',
        gap: '8px 20px',
      }}
    >
      <Box sx={{order: {xs: 2, md: 1}, px: 1.25, py: {xs: 0.5, md: 1}}}>
        <Typography
          component="span"
          sx={{
            display: 'block',
            fontFamily: 'var(--ifm-font-family-monospace)',
            fontSize: '0.72rem',
            letterSpacing: '0.06em',
            textTransform: 'uppercase',
            color: t.cyan,
          }}
        >
          Build
        </Typography>
        <Typography
          component="h3"
          sx={{m: 0, mt: 0.75, fontFamily: DISPLAY_FONT, fontSize: '1.0625rem', fontWeight: 600}}
        >
          SDKs
        </Typography>
        <Typography sx={{mt: 0.75, fontSize: '0.9rem', lineHeight: 1.55, color: 'text.secondary'}}>
          Starter SDKs and framework integrations
        </Typography>
        <TextLink href="/sdks-and-tools" sx={{mt: 1.5, fontSize: '0.875rem'}}>
          All SDKs &amp; tools
        </TextLink>
      </Box>

      <Box
        sx={{
          order: {xs: 1, md: 2},
          display: 'flex',
          flexDirection: 'column',
          gap: 1.5,
          p: '16px 18px',
          minHeight: 118,
          justifyContent: 'center',
          borderRadius: '12px',
          bgcolor: t.inset,
          border: '1px solid',
          borderColor: t.line,
        }}
      >
        <Box
          component="span"
          sx={{
            alignSelf: 'flex-start',
            px: 1.25,
            py: 0.375,
            borderRadius: '999px',
            fontSize: '0.72rem',
            fontWeight: 600,
            color: t.cyan,
            bgcolor: 'rgba(0,213,255,0.1)',
            border: '1px solid rgba(0,213,255,0.28)',
          }}
        >
          {SDKS.length}+ official SDKs
        </Box>
        <Box
          component="ul"
          aria-label="Official SDKs"
          sx={{
            listStyle: 'none',
            m: 0,
            p: 0,
            display: 'grid',
            gridTemplateColumns: {
              xs: 'repeat(2, minmax(0, 1fr))',
              sm: 'repeat(4, minmax(0, 1fr))',
              lg: 'repeat(5, minmax(0, 1fr))',
            },
            gap: 1,
            '& > li:nth-of-type(n+5)': {display: {xs: 'none', lg: 'block'}},
          }}
        >
          {featured.map((sdk) => (
            <li key={sdk.name}>
              <Box
                component={Link}
                to={docsUrl(sdk.href)}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  height: 46,
                  px: 1.125,
                  borderRadius: '10px',
                  border: '1px solid',
                  borderColor: t.line,
                  bgcolor: t.isDark ? 'rgba(4,12,26,0.6)' : '#fff',
                  color: 'text.primary',
                  fontSize: '0.84rem',
                  fontWeight: 600,
                  textDecoration: 'none',
                  transition: 'border-color 0.2s, background-color 0.2s',
                  '&:hover': {
                    borderColor: 'rgba(54,136,255,0.55)',
                    bgcolor: t.isDark ? 'rgba(8,32,64,0.7)' : 'rgba(54,136,255,0.05)',
                    color: 'text.primary',
                    textDecoration: 'none',
                  },
                }}
              >
                <Box
                  component="span"
                  sx={{
                    flex: 'none',
                    width: 28,
                    height: 28,
                    borderRadius: '7px',
                    display: 'grid',
                    placeItems: 'center',
                    bgcolor: t.isDark ? 'rgba(200,225,255,0.06)' : 'rgba(15,40,80,0.04)',
                    border: '1px solid',
                    borderColor: t.line,
                  }}
                >
                  <sdk.icon size={18} />
                </Box>
                <Box
                  component="span"
                  sx={{minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}
                >
                  {sdk.name}
                </Box>
              </Box>
            </li>
          ))}
        </Box>
      </Box>
    </Box>
  );
}
