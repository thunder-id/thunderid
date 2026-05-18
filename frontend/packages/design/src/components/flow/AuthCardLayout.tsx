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

import {cn} from '@thunderid/utils';
import {ColorSchemeImage, Paper, Stack, styled} from '@wso2/oxygen-ui';
import type {JSX, ReactNode} from 'react';

const StyledPaper = styled(Paper)(({theme}) => ({
  display: 'flex',
  flexDirection: 'column',
  alignSelf: 'center',
  width: '100%',
  padding: theme.spacing(4),
  gap: theme.spacing(2),
  [theme.breakpoints.up('sm')]: {
    width: '450px',
  },
}));

export interface AuthCardLayoutProps {
  /** Class name prefix for product name prefixed CSS (e.g. 'SignInBox' → '<PRODUCT_NAME>SignInBox--root'). */
  variant?: string;
  /** Logo image sources for light/dark modes. */
  logo?: {
    src: {light: string; dark: string};
    alt?: {light: string; dark: string};
  };
  /** Whether to show the logo. Defaults to true. */
  showLogo?: boolean;
  /** Custom sx for the logo's display behavior. Defaults to mobile-only. */
  logoDisplay?: Record<string, string>;
  children: ReactNode;
}

export default function AuthCardLayout({
  variant = undefined,
  logo = undefined,
  showLogo = true,
  logoDisplay = {xs: 'flex', md: 'none'},
  children,
}: AuthCardLayoutProps): JSX.Element {
  return (
    <Stack gap={2} className={variant ? cn(`${variant}--root`) : undefined}>
      {showLogo && logo && (
        <ColorSchemeImage
          className={variant ? cn(`${variant}--logo`) : undefined}
          src={logo.src}
          alt={logo.alt ?? {light: 'Logo (Light)', dark: 'Logo (Dark)'}}
          height={40}
          width="auto"
          sx={{display: logoDisplay}}
        />
      )}
      <StyledPaper variant="outlined" className={variant ? cn(`${variant}--paper`) : undefined}>
        {children}
      </StyledPaper>
    </Stack>
  );
}
