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

import ProductName from '@site/src/components/ProductName';
import {Box, ColorSchemeImage, Container, Link, Typography, useTheme} from '@wso2/oxygen-ui';
import {WSO2} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import useScrollAnimation from '../../hooks/useScrollAnimation';

export default function FooterSection(): JSX.Element {
  const isDark = useIsDarkMode();
  const theme = useTheme();
  const {ref, isVisible} = useScrollAnimation({threshold: 0.2});

  return (
    <Box
      sx={{
        py: {xs: 8, lg: 12},
        position: 'relative',
        borderTop: '1px solid',
        borderColor: 'divider',
        '&::before': {
          content: '""',
          position: 'absolute',
          top: 0,
          left: 0,
          right: 0,
          bottom: 0,
          background: isDark
            ? `radial-gradient(ellipse at 50% 50%, rgba(${theme.vars?.palette.primary.main} / 0.07) 0%, transparent 60%)`
            : `radial-gradient(ellipse at 50% 50%, rgba(${theme.vars?.palette.primary.main} / 0.04) 0%, transparent 60%)`,
          pointerEvents: 'none',
        },
      }}
    >
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}, position: 'relative', zIndex: 1}}>
        <Box
          ref={ref}
          sx={{
            textAlign: 'center',
            maxWidth: '720px',
            mx: 'auto',
            opacity: isVisible ? 1 : 0,
            transform: isVisible ? 'translateY(0)' : 'translateY(32px)',
            transition: 'opacity 0.7s cubic-bezier(0.16, 1, 0.3, 1), transform 0.7s cubic-bezier(0.16, 1, 0.3, 1)',
          }}
        >
          <Typography
            variant="h4"
            sx={{
              lineHeight: 1.7,
              color: 'text.secondary',
              mb: 3,
            }}
          >
            <ProductName /> is an{' '}
            <Link
              href="https://openwallet.foundation/"
              target="_blank"
              rel="noopener noreferrer"
              sx={{
                background: `linear-gradient(90deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
                fontWeight: 600,
                textDecoration: 'none',
                '&:hover': {
                  textDecoration: 'underline',
                },
              }}
            >
              Open Wallet Foundation (OWF)
            </Link>{' '}
            project
          </Typography>

          <Box
            sx={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              mb: 3,
              minHeight: '60px',
            }}
          >
            <ColorSchemeImage
              src={{
                light: '/assets/images/openwallet-foundation-logo-color.svg',
                dark: '/assets/images/openwallet-foundation-logo-white.svg',
              }}
              height={150}
              width={'auto'}
              alt="Open Wallet Foundation Logo"
            />
          </Box>

          <Box
            sx={{
              display: 'flex',
              justifyContent: 'center',
              alignItems: 'center',
              gap: 0.5,
            }}
          >
            <Typography
              variant="body2"
              sx={{
                fontSize: {xs: '0.85rem', sm: '0.9rem'},
                color: 'text.secondary',
              }}
            >
              Made with ❤️ by
            </Typography>
            <WSO2 size={16} sx={{verticalAlign: 'middle'}} />
            <Typography
              variant="body2"
              sx={{
                fontSize: {xs: '0.85rem', sm: '0.9rem'},
                color: 'text.secondary',
              }}
            >
              WSO2
            </Typography>
          </Box>
        </Box>
      </Container>
    </Box>
  );
}
