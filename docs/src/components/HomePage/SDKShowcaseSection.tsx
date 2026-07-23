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
import {Box, Container, Typography} from '@wso2/oxygen-ui';
import {JSX, useState} from 'react';
import AndroidLogo from '../icons/AndroidLogo';
import ExpressLogo from '../icons/ExpressLogo';
import FlutterLogo from '../icons/FlutterLogo';
import IOSLogo from '../icons/IOSLogo';
import JavaScriptLogo from '../icons/JavaScriptLogo';
import NextLogo from '../icons/NextLogo';
import NodeLogo from '../icons/NodeLogo';
import NuxtLogo from '../icons/NuxtLogo';
import ReactLogo from '../icons/ReactLogo';
import VueLogo from '../icons/VueLogo';
import useIsDarkMode from '@site/src/hooks/useIsDarkMode';

const SDKS = [
  {
    name: 'React',
    packageName: '@thunderid/react',
    icon: ReactLogo,
    href: '/docs/next/getting-started/connect-your-application/react',
  },
  {
    name: 'Next.js',
    packageName: '@thunderid/nextjs',
    icon: NextLogo,
    href: '/docs/next/getting-started/connect-your-application/nextjs',
  },
  {
    name: 'Express',
    packageName: '@thunderid/express',
    icon: ExpressLogo,
    href: '/docs/next/getting-started/connect-your-application/express',
  },
  {
    name: 'Vue',
    packageName: '@thunderid/vue',
    icon: VueLogo,
    href: '/docs/next/getting-started/connect-your-application/vue',
  },
  {
    name: 'Nuxt',
    packageName: '@thunderid/nuxt',
    icon: NuxtLogo,
    href: '/docs/next/getting-started/connect-your-application/nuxt',
  },
  {
    name: 'Node.js',
    packageName: '@thunderid/node',
    icon: NodeLogo,
    href: '/docs/next/getting-started/connect-your-application/node',
  },
  {
    name: 'Vanilla JavaScript',
    packageName: '@thunderid/browser',
    icon: JavaScriptLogo,
    href: '/docs/next/getting-started/connect-your-application/browser',
  },
  {
    name: 'iOS',
    packageName: 'ThunderID',
    icon: IOSLogo,
    href: '/docs/next/getting-started/connect-your-application/ios',
  },
  {
    name: 'Android',
    packageName: 'dev.thunderid:compose',
    icon: AndroidLogo,
    href: '/docs/next/getting-started/connect-your-application/android',
  },
  {
    name: 'Flutter',
    packageName: 'thunderid_flutter',
    icon: FlutterLogo,
    href: '/docs/next/getting-started/connect-your-application/flutter',
  },
];

export default function SDKShowcaseSection(): JSX.Element {
   
  const [hoveredIndex, rawSet] = useState<number | null>(null);
  const isDark = useIsDarkMode();
  const setHoveredIndex = rawSet as (v: number | null) => void;
  const isHovering = hoveredIndex !== null;
  const hoveredSdk = SDKS.find((_, i) => i === hoveredIndex);
  const displayName = hoveredSdk?.name ?? '';

  return (
    <Box
      sx={{
        py: {xs: 4, md: 5},
        borderTop: '1px solid',
        borderColor: 'divider',
        '@keyframes sdkFadeIn': {
          from: {opacity: 0, transform: 'translateY(16px)'},
          to: {opacity: 1, transform: 'translateY(0)'},
        },
        background: isDark ? 'rgba(10, 10, 10, 0.2)' : 'rgba(255, 255, 255, 0.2)',
        animation: 'sdkFadeIn 0.6s cubic-bezier(0.16, 1, 0.3, 1) 0.6s both',
      }}
    >
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}}}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'},
            gap: {xs: 4, md: 4},
            alignItems: 'center',
          }}
        >
          {/* Left: slot-machine text */}
          <Box sx={{textAlign: {xs: 'center', md: 'left'}}}>
            <Typography
              variant="h2"
              sx={{
                color: 'text.secondary',
                lineHeight: 1.3,
              }}
            >
              Use ThunderID with
            </Typography>

            <Box sx={{height: '4rem', overflow: 'hidden', position: 'relative', mt: 0.5}}>
              {/* "any framework" — exits upward on hover */}
              <Box
                sx={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: {xs: 'center', md: 'flex-start'},
                  transition: 'transform 0.35s cubic-bezier(0.7, 0, 0.3, 1), opacity 0.25s ease',
                  transform: isHovering ? 'translateY(-110%)' : 'translateY(0)',
                  opacity: isHovering ? 0 : 1,
                  pointerEvents: 'none',
                  userSelect: 'none',
                }}
              >
                <Typography
                  component="h1"
                  sx={{
                    fontWeight: 700,
                    fontSize: {xs: '2.5rem', md: '3rem'},
                    lineHeight: 1,
                    color: 'text.primary',
                    whiteSpace: 'nowrap',
                  }}
                >
                  any framework
                </Typography>
              </Box>

              {/* SDK name — rises on hover */}
              <Box
                sx={{
                  position: 'absolute',
                  inset: 0,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: {xs: 'center', md: 'flex-start'},
                  transition: 'transform 0.35s cubic-bezier(0.7, 0, 0.3, 1), opacity 0.25s ease',
                  transform: isHovering ? 'translateY(0)' : 'translateY(110%)',
                  opacity: isHovering ? 1 : 0,
                  pointerEvents: 'none',
                  userSelect: 'none',
                }}
              >
                <Typography
                  component="h1"
                  sx={{
                    fontWeight: 700,
                    fontSize: {xs: '2.5rem', md: '3rem'},
                    lineHeight: 1,
                    color: 'text.primary',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {displayName}
                </Typography>
              </Box>
            </Box>
          </Box>

          {/* Right: icon grid */}
          <Box>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: 'repeat(5, 1fr)',
                gap: {xs: 1.5, md: 2},
              }}
            >
              {SDKS.map((sdk, index) => {
                const Icon = sdk.icon;
                const isActive = hoveredIndex === index;

                return (
                  <Link key={sdk.name} to={sdk.href} title={sdk.name} style={{textDecoration: 'none', display: 'block'}}>
                    <Box
                      onMouseEnter={() => {
                        setHoveredIndex(index);
                      }}
                      onMouseLeave={() => {
                        setHoveredIndex(null);
                      }}
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: '100%',
                        aspectRatio: '1',
                        borderRadius: '10px',
                        border: '1px solid',
                        borderColor: isActive ? 'rgba(255,255,255,0.25)' : 'divider',
                        bgcolor: isActive ? 'rgba(255,255,255,0.05)' : 'transparent',
                        transform: isActive ? 'scale(1.06)' : 'scale(1)',
                        transition: [
                          'border-color 0.2s ease',
                          'background-color 0.2s ease',
                          'filter 0.2s ease',
                          'transform 0.2s cubic-bezier(0.34, 1.56, 0.64, 1)',
                        ].join(', '),
                        cursor: 'pointer',
                      }}
                    >
                      <Icon size={32} />
                    </Box>
                  </Link>
                );
              })}
            </Box>
          </Box>
        </Box>
      </Container>
    </Box>
  );
}
