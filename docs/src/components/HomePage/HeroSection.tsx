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
import {Box, Container, Typography, Stack, Button, useTheme} from '@wso2/oxygen-ui';
import React, {JSX} from 'react';
import ConstellationBackground from './ConstellationBackground';

export default function HeroSection(): JSX.Element {
  const theme = useTheme();

  return (
    <Box
      sx={{
        '@keyframes fadeInUp': {
          from: {opacity: 0, transform: 'translateY(32px)'},
          to: {opacity: 1, transform: 'translateY(0)'},
        },
        '@keyframes pulseGlow': {
          '0%, 100%': {opacity: 0.6, transform: 'scale(1)'},
          '50%': {opacity: 1, transform: 'scale(1.1)'},
        },
        '@keyframes heroFloat': {
          '0%, 100%': {transform: 'translateY(0)'},
          '50%': {transform: 'translateY(-6px)'},
        },
        py: {xs: 7, lg: 10},
        position: 'relative',
        overflow: 'hidden',
        background: 'transparent',
      }}
    >
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}, position: 'relative', zIndex: 1}}>
        <Box
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            py: {xs: 5, lg: 8},
            textAlign: 'center',
          }}
        >
          {/* Lightning bolt icon */}
          <Box
            sx={{
              mb: 3,
              position: 'relative',
              width: 80,
              height: 120,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) both',
            }}
          >
            <Box
              sx={{
                position: 'absolute',
                width: 120,
                height: 120,
                borderRadius: '50%',
                background: `radial-gradient(circle, rgba(${theme.vars?.palette.primary.main} / 0.25) 0%, transparent 70%)`,
                filter: 'blur(20px)',
                animation: 'pulseGlow 3s ease-in-out infinite',
              }}
            />
            <svg
              width="56"
              height="80"
              viewBox="0 0 24 32"
              fill="none"
              style={{position: 'relative', zIndex: 1, animation: 'heroFloat 4s ease-in-out infinite'}}
            >
              <path
                d="M13.5 1L4 18h7l-1.5 13L20 14h-7L13.5 1z"
                stroke={theme.vars?.palette.primary.main}
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
                fill="none"
              />
            </svg>
          </Box>

          {/* INTRODUCING label */}
          <Typography
            variant="overline"
            sx={{
              mb: 1.5,
              fontSize: '0.8rem',
              letterSpacing: '0.25em',
              color: theme.vars?.palette.primary.main,
              fontWeight: 500,
              animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.1s both',
            }}
          >
            INTRODUCING
          </Typography>

          {/* [ THUNDER ] title */}
          <Typography
            variant="h2"
            sx={{
              mb: 3,
              fontSize: {xs: '2rem', sm: '2.5rem', md: '3rem'},
              fontWeight: 300,
              letterSpacing: '0.15em',
              color: 'text.primary',
              animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.2s both',
            }}
          >
            [ ThunderID ]
          </Typography>

          {/* Main heading */}
          <Typography
            variant="h1"
            sx={{
              mb: 3,
              fontSize: {xs: '2.75rem', sm: '3.5rem', md: '4.5rem'},
              fontWeight: 700,
              lineHeight: 1.1,
              color: 'text.primary',
              animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.3s both',
            }}
          >
            <Box
              component="span"
              sx={{
                background: `linear-gradient(90deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
                backgroundClip: 'text',
              }}
            >
              Auth
            </Box>{' '}
            for the Modern Dev
          </Typography>

          {/* Description */}
          <Typography
            variant="body1"
            sx={{
              maxWidth: '680px',
              textAlign: 'center',
              mb: 5,
              fontSize: {xs: '1rem', sm: '1.15rem'},
              lineHeight: 1.7,
              color: 'text.secondary',
              animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.4s both',
            }}
          >
            High-performance open-source identity stack, engineered for developers.
          </Typography>

          {/* Buttons */}
          <Stack
            direction={{xs: 'column', sm: 'row'}}
            spacing={2}
            sx={{mb: 8, animation: 'fadeInUp 0.8s cubic-bezier(0.16, 1, 0.3, 1) 0.5s both'}}
            alignItems="center"
          >
            <Button
              component={Link}
              href="/docs/next/guides/getting-started/get-thunderid"
              variant="contained"
              color="primary"
              size="large"
              sx={{
                px: 5,
                py: 1.5,
                fontWeight: 600,
                textTransform: 'none',
                fontSize: '1.05rem',
                borderRadius: '28px',
                background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                position: 'relative',
                overflow: 'hidden',
                transition: 'transform 0.3s ease, box-shadow 0.3s ease',
                '&::after': {
                  content: '""',
                  position: 'absolute',
                  top: 0,
                  left: '-100%',
                  width: '60%',
                  height: '100%',
                  background: 'linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.2), transparent)',
                  transition: 'none',
                  transform: 'skewX(-15deg)',
                },
                '&:hover::after': {
                  left: '150%',
                  transition: 'left 0.6s ease',
                },
                '&:hover': {
                  background: `linear-gradient(135deg, ${theme.vars?.palette.primary.dark} 0%, ${theme.vars?.palette.primary.main} 100%)`,
                  transform: 'translateY(-2px)',
                  boxShadow: `0 6px 24px rgba(${theme.vars?.palette.primary.main} / 0.35), 0 0 40px rgba(${theme.vars?.palette.primary.main} / 0.1)`,
                },
                '&:active': {
                  transform: 'translateY(0)',
                  boxShadow: `0 2px 8px rgba(${theme.vars?.palette.primary.main} / 0.2)`,
                },
              }}
            >
              Start Building
            </Button>
            <Button
              component={Link}
              href="/docs/next/guides/getting-started/what-is-thunderid"
              variant="outlined"
              size="large"
              sx={{
                px: 4,
                py: 1.5,
                textTransform: 'none',
                fontSize: '1.05rem',
                borderRadius: '28px',
                borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.45)`,
                color: 'primary.main',
                position: 'relative',
                overflow: 'hidden',
                transition:
                  'transform 0.3s ease, border-color 0.3s ease, box-shadow 0.3s ease, background-color 0.3s ease',
                '&::before': {
                  content: '""',
                  position: 'absolute',
                  inset: 0,
                  borderRadius: 'inherit',
                  background: `radial-gradient(circle at center, rgba(${theme.vars?.palette.primary.main} / 0.07) 0%, transparent 70%)`,
                  opacity: 0,
                  transition: 'opacity 0.3s ease',
                },
                '&:hover::before': {opacity: 1},
                '&:hover': {
                  borderColor: `rgba(${theme.vars?.palette.primary.main} / 0.7)`,
                  bgcolor: `rgba(${theme.vars?.palette.primary.main} / 0.05)`,
                  transform: 'translateY(-2px)',
                  boxShadow: `0 4px 16px rgba(${theme.vars?.palette.primary.main} / 0.12), 0 0 0 1px rgba(${theme.vars?.palette.primary.main} / 0.15)`,
                },
                '&:active': {transform: 'translateY(0)', boxShadow: 'none'},
              }}
            >
              Learn More
            </Button>
          </Stack>
        </Box>
      </Container>
    </Box>
  );
}
