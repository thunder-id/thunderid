// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {SloganItemConfig} from '@thunderid/contexts';
import {cn} from '@thunderid/utils';
import {ColorSchemeImage, Stack, Typography} from '@wso2/oxygen-ui';
import {Bot, ShieldCheck, Wallet, Zap} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

const items: {
  icon: JSX.Element;
  title: string;
  description: string;
}[] = [
  {
    icon: <Bot className="text-muted-foreground" />,
    title: 'Agent-native Identity',
    description:
      'Engineered with native Agent ID to secure end-to-end workflows among humans, AI agents, and machines.',
  },
  {
    icon: <Wallet className="text-muted-foreground" />,
    title: 'Decentralized Identity',
    description:
      'Standards-based support for DIDs, verifiable credentials, and digital wallets for user-controlled identity.',
  },
  {
    icon: <ShieldCheck className="text-muted-foreground" />,
    title: 'Post-quantum-safe by Design',
    description: 'Built on a post-quantum cryptographic foundation to be inherently resistant to attacks by design.',
  },
  {
    icon: <Zap className="text-muted-foreground" />,
    title: 'Lightweight Runtime with GitOps Support',
    description:
      'Cloud-native, API-first runtime that integrates into modern CI/CD, GitOps, and containerized workflows.',
  },
];

export default function SignInSlogan(): JSX.Element {
  const slogan = window.__THUNDERID_RUNTIME_CONFIG__?.brand?.slogan;
  let configuredItems: SloganItemConfig[] = [];
  if (typeof slogan === 'string') {
    configuredItems = slogan.trim() !== '' ? [{title: slogan}] : [];
  } else if (Array.isArray(slogan)) {
    configuredItems = slogan;
  } else if (typeof slogan === 'object' && slogan !== null) {
    configuredItems = [slogan];
  }

  const displayItems: {icon?: JSX.Element; title: string; description?: string}[] =
    configuredItems.length > 0
      ? configuredItems.map((item) => ({
          icon: item.icon ? (
            <img src={`${import.meta.env.BASE_URL.replace(/\/$/, '')}/${item.icon}`} alt="" width={24} height={24} />
          ) : undefined,
          title: item.title,
          description: item.description,
        }))
      : items;

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
        {displayItems.map((item) => (
          <Stack key={item.title} direction="row" sx={{gap: 2}}>
            {item.icon}
            <div>
              <Typography gutterBottom sx={{fontWeight: 'medium'}}>
                {item.title}
              </Typography>
              {item.description && (
                <Typography variant="body2" sx={{color: 'text.secondary'}}>
                  {item.description}
                </Typography>
              )}
            </div>
          </Stack>
        ))}
      </Stack>
    </Stack>
  );
}
