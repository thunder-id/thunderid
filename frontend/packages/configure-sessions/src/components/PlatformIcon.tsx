// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AndroidIcon,
  AppleIcon,
  ChromeIcon,
  FirefoxIcon,
  LinuxIcon,
  OperaIcon,
  SafariIcon,
  WindowsIcon,
} from '@thunderid/components';
import {Box} from '@wso2/oxygen-ui';
import {Globe, Monitor} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';

type Logo = (props: {size?: number}) => JSX.Element;

// Names from describeUserAgent that have a logo. The others get a generic icon: there is no mark for Edge or
// Samsung Internet.
const BROWSER_LOGOS: Record<string, Logo> = {
  Chrome: ChromeIcon,
  Firefox: FirefoxIcon,
  Opera: OperaIcon,
  Safari: SafariIcon,
};
const OS_LOGOS: Record<string, Logo> = {
  macOS: AppleIcon,
  iOS: AppleIcon,
  Android: AndroidIcon,
  Linux: LinuxIcon,
  Windows: WindowsIcon,
  ChromeOS: ChromeIcon,
};

interface PlatformIconProps {
  /** A browser or operating system name as describeUserAgent returns it. */
  name?: string;
  kind: 'browser' | 'os';
  size?: number;
}

/**
 * The logo of a browser or operating system, or a generic icon for one without a logo here.
 */
export default function PlatformIcon({name = undefined, kind, size = 16}: PlatformIconProps): JSX.Element {
  const logos = kind === 'browser' ? BROWSER_LOGOS : OS_LOGOS;
  const Logo = name ? logos[name] : undefined;
  if (Logo) {
    return (
      <Box component="span" aria-hidden sx={{display: 'flex'}}>
        <Logo size={size} />
      </Box>
    );
  }
  if (kind === 'browser') {
    return <Globe size={size} aria-hidden />;
  }
  return <Monitor size={size} aria-hidden />;
}
