/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {cn} from '@thunderid/utils';
import {ColorSchemeImage, Stack, Typography} from '@wso2/oxygen-ui';
import {Cloud, ShieldCheck, Zap, TerminalSquare} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

const items: {
  icon: JSX.Element;
  title: string;
  description: string;
}[] = [
  {
    icon: <Cloud className="text-muted-foreground" />,
    title: 'Flexible Identity Platform',
    description: 'Centralizes identity management for both on-prem and cloud environments with no protocol lock-in.',
  },
  {
    icon: <ShieldCheck className="text-muted-foreground" />,
    title: 'Zero-trust Security',
    description: 'Leverage adaptive authentication, OIDC, and OAuth 2.0 to protect every login and session.',
  },
  {
    icon: <TerminalSquare className="text-muted-foreground" />,
    title: 'Developer-first Experience',
    description: 'Configure auth flows and manage organizations with powerful SDKs and APIs.',
  },
  {
    icon: <Zap className="text-muted-foreground" />,
    title: 'Extensible & Enterprise-ready',
    description: 'Built for scale, integrates with your stack and CI/CD pipelines, and ready for any cloud.',
  },
];

export default function SignInSlogan(): JSX.Element {
  const logoSrc = {
    light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
    dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
  };

  return (
    <Stack
      direction="column"
      alignItems="start"
      gap={5}
      maxWidth={450}
      display={{xs: 'none', md: 'flex'}}
      className={cn('SignInSlogan--root')}
    >
      <ColorSchemeImage src={logoSrc} alt={{light: 'Logo (Light)', dark: 'Logo (Dark)'}} height={50} width="auto" />
      <Stack sx={{flexDirection: 'column', alignSelf: 'center', gap: 4}}>
        {items.map((item) => (
          <Stack key={item.title} direction="row" sx={{gap: 2}}>
            {item.icon}
            <div>
              <Typography gutterBottom sx={{fontWeight: 'medium'}}>
                {item.title}
              </Typography>
              <Typography variant="body2" sx={{color: 'text.secondary'}}>
                {item.description}
              </Typography>
            </div>
          </Stack>
        ))}
      </Stack>
    </Stack>
  );
}
