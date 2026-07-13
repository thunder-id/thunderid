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
import {JSX} from 'react';
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

const LOGO_SIZE = 30;

const ORDER = [
  ReactLogo,
  NextLogo,
  VueLogo,
  NuxtLogo,
  BrowserLogo,
  ExpressLogo,
  NodeLogo,
  GoLogo,
  PythonLogo,
  IOSLogo,
  AndroidLogo,
  FlutterLogo,
  AngularLogo,
  ReactRouterLogo,
];

const CLOUD = Array.from({length: 22}, (_, i) => {
  const Logo = ORDER[i % ORDER.length];
  const pass = Math.floor(i / ORDER.length);
  return {Logo, key: `${Logo.name}-${pass}`};
});

export default function FloatingLogosBackground(): JSX.Element {
  return (
    <Box
      sx={{
        position: 'absolute',
        top: 0,
        left: 0,
        right: 0,
        height: 280,
        overflow: 'hidden',
        pointerEvents: 'none',
        maskImage: 'linear-gradient(black 0%, rgba(0,0,0,0.5) 55%, transparent 100%)',
        WebkitMaskImage: 'linear-gradient(black 0%, rgba(0,0,0,0.5) 55%, transparent 100%)',
      }}
    >
      <Box
        sx={{
          display: 'flex',
          flexWrap: 'wrap',
          justifyContent: 'center',
          gap: '34px',
          p: '34px 40px',
          filter: 'grayscale(1)',
          opacity: 0.13,
        }}
      >
        {CLOUD.map(({Logo, key}) => (
          <Box key={key} sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', width: 30, height: 30}}>
            <Logo size={LOGO_SIZE} />
          </Box>
        ))}
      </Box>
    </Box>
  );
}
