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

import {Box} from '@wso2/oxygen-ui';
import React, {JSX, useMemo} from 'react';
import AndroidLogo from './icons/AndroidLogo';
import AngularLogo from './icons/AngularLogo';
import BrowserLogo from './icons/BrowserLogo';
import ExpressLogo from './icons/ExpressLogo';
import FlutterLogo from './icons/FlutterLogo';
import GoLogo from './icons/GoLogo';
import IOSLogo from './icons/IOSLogo';
import NextLogo from './icons/NextLogo';
import NodeLogo from './icons/NodeLogo';
import NuxtLogo from './icons/NuxtLogo';
import PythonLogo from './icons/PythonLogo';
import ReactLogo from './icons/ReactLogo';
import ReactRouterLogo from './icons/ReactRouterLogo';
import VueLogo from './icons/VueLogo';
import useIsDarkMode from '../hooks/useIsDarkMode';

const LOGO_SIZE = 28;

const ALL_LOGOS = [
  {Component: ReactLogo, name: 'React'},
  {Component: NextLogo, name: 'Next.js'},
  {Component: VueLogo, name: 'Vue'},
  {Component: NuxtLogo, name: 'Nuxt'},
  {Component: AngularLogo, name: 'Angular'},
  {Component: NodeLogo, name: 'Node.js'},
  {Component: ExpressLogo, name: 'Express'},
  {Component: GoLogo, name: 'Go'},
  {Component: PythonLogo, name: 'Python'},
  {Component: FlutterLogo, name: 'Flutter'},
  {Component: IOSLogo, name: 'iOS'},
  {Component: AndroidLogo, name: 'Android'},
  {Component: BrowserLogo, name: 'Browser'},
  {Component: ReactRouterLogo, name: 'React Router'},
];

// Create rows with different logo orderings for visual variety.
const ROWS = [
  [0, 3, 6, 9, 12, 1, 4, 7, 10, 13, 2, 5, 8, 11],
  [13, 10, 7, 4, 1, 12, 9, 6, 3, 0, 11, 8, 5, 2],
  [2, 5, 8, 11, 0, 3, 6, 9, 12, 1, 4, 7, 10, 13],
].map((indices) => indices.map((i) => ALL_LOGOS[i]));

// Deterministic pseudo-random delays so they're stable across renders.
// Seeded from row + position to avoid layout shift.
function staggerDelay(rowIdx: number, itemIdx: number): number {
  const seed = (rowIdx * 28 + itemIdx * 17 + 7) % 100;
  return (seed / 100) * 1.8; // 0 – 1.8s
}

interface LogoRowProps {
  logos: typeof ALL_LOGOS;
  rowIndex: number;
  direction: 'left' | 'right';
  duration: number;
  isDark: boolean;
}

function LogoRow({logos, rowIndex, direction, duration, isDark}: LogoRowProps): JSX.Element {
  const animName = direction === 'left' ? 'scrollLeft' : 'scrollRight';

  // Triple logos to ensure full coverage across all viewport widths.
  // scrollLeft: 0 → -33%, scrollRight: -33% → 0 (both show the middle third initially).
  const tripled = useMemo(() => [...logos, ...logos, ...logos], [logos]);

  return (
    <Box
      sx={{
        overflow: 'hidden',
        py: 0.75,
        position: 'relative',
        maskImage: 'linear-gradient(to right, transparent 0%, black 8%, black 92%, transparent 100%)',
        WebkitMaskImage: 'linear-gradient(to right, transparent 0%, black 8%, black 92%, transparent 100%)',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          gap: 2,
          width: 'max-content',
          animation: `${animName} ${duration}s linear infinite`,
        }}
      >
        {tripled.map((logo, i) => (
          <Box
            key={`${logo.name}-${i}`}
            sx={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              width: 44,
              height: 44,
              borderRadius: '10px',
              background: isDark ? 'rgba(255, 255, 255, 0.04)' : 'rgba(0, 0, 0, 0.03)',
              border: `1px solid ${isDark ? 'rgba(255, 255, 255, 0.06)' : 'rgba(0, 0, 0, 0.05)'}`,
              flexShrink: 0,
              opacity: 'var(--logo-opacity)',
              animation: `logoFadeIn 0.8s ease-out ${staggerDelay(rowIndex, i)}s backwards`,
            }}
          >
            <logo.Component size={LOGO_SIZE} />
          </Box>
        ))}
      </Box>
    </Box>
  );
}

export default function FloatingLogosBackground(): JSX.Element {
  const isDark = useIsDarkMode();
  const targetOpacity = isDark ? 0.35 : 0.3;

  return (
    <Box
      sx={{
        '@keyframes scrollLeft': {
          '0%': {transform: 'translateX(0)'},
          '100%': {transform: 'translateX(calc(-100% / 3))'},
        },
        '@keyframes scrollRight': {
          '0%': {transform: 'translateX(calc(-100% / 3))'},
          '100%': {transform: 'translateX(0)'},
        },
        [`@keyframes logoFadeIn`]: {
          '0%': {opacity: 0, transform: 'scale(0.85)'},
          '100%': {opacity: 'var(--logo-opacity)', transform: 'scale(1)'},
        },
        '--logo-opacity': targetOpacity,
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        bottom: 0,
        overflow: 'hidden',
        pointerEvents: 'none',
        zIndex: 0,
        opacity: 0.5,
      }}
    >
      {/* Radial gradient overlay to fade logos near center where text sits */}
      <Box
        sx={{
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          zIndex: 1,
          background: isDark
            ? 'radial-gradient(ellipse 70% 60% at 50% 55%, rgba(10, 10, 10, 0.95) 0%, rgba(10, 10, 10, 0.6) 40%, transparent 70%)'
            : 'radial-gradient(ellipse 70% 60% at 50% 55%, rgba(255, 255, 255, 0.95) 0%, rgba(255, 255, 255, 0.6) 40%, transparent 70%)',
          pointerEvents: 'none',
        }}
      />
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'flex-start',
          gap: 0.5,
          pt: 2,
          height: '100%',
        }}
      >
        {ROWS.map((logos, i) => (
          <LogoRow
            key={i}
            logos={logos}
            rowIndex={i}
            direction={i % 2 === 0 ? 'left' : 'right'}
            duration={60 + i * 10}
            isDark={isDark}
          />
        ))}
      </Box>
    </Box>
  );
}
