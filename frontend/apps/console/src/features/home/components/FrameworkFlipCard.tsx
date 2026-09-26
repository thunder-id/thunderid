// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {TechnologyBasedApplicationTemplateMetadata} from '@thunderid/configure-applications';
import {Box, Stack, Typography, useColorScheme} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';

const SLOT_COUNT = 3;
const ICON_SIZE = 24;
const CARD_WIDTH = 176;
const CARD_HEIGHT = 52;

// Each slot flips at its own pace so the three cards drift in and out of sync rather than
// flipping in lockstep.
const SLOT_STAGGER_SECONDS = [3, 4.2, 5.4];
const FADE_SECONDS = 0.4;
// Fading out slightly past the next card's start (instead of ending exactly on it) keeps a
// sliver of the outgoing card visible while the incoming one fades in, so there's never a
// fully blank instant at the crossfade boundary.
const OVERLAP_SECONDS = 0.1;

// Reuses the same template metadata that powers the "Create Application" technology picker,
// so any SDK added there shows up here automatically — no separate list to keep in sync.
const AVAILABLE_TEMPLATES = TechnologyBasedApplicationTemplateMetadata.filter((template) => !template.disabled);

// Splits the templates across SLOT_COUNT independent flip-cards so several are visible at
// once (e.g. React and Vue can be showing side by side instead of one at a time).
const SLOTS = Array.from({length: SLOT_COUNT}, (_, slotIndex) =>
  AVAILABLE_TEMPLATES.filter((_, index) => index % SLOT_COUNT === slotIndex),
);

function keyframePercent(seconds: number, cycleSeconds: number): number {
  return (seconds / cycleSeconds) * 100;
}

interface FlipSlotProps {
  templates: typeof AVAILABLE_TEMPLATES;
  slotIndex: number;
  staggerSeconds: number;
  isDark: boolean;
  onSelect: (templateId: string) => void;
}

function FlipSlot({templates, slotIndex, staggerSeconds, isDark, onSelect}: FlipSlotProps): JSX.Element {
  const {t} = useTranslation();
  const cycleSeconds = templates.length * staggerSeconds;
  const fadeOutSeconds = staggerSeconds + OVERLAP_SECONDS;
  const holdEndSeconds = fadeOutSeconds - FADE_SECONDS;
  const animationName = `frameworkFlip${slotIndex}`;

  return (
    <Box
      sx={{
        [`@keyframes ${animationName}`]: {
          '0%': {opacity: 0, transform: 'rotateX(-100deg)', pointerEvents: 'none'},
          [`${keyframePercent(FADE_SECONDS, cycleSeconds)}%`]: {
            opacity: 1,
            transform: 'rotateX(0deg)',
            pointerEvents: 'auto',
          },
          [`${keyframePercent(holdEndSeconds, cycleSeconds)}%`]: {
            opacity: 1,
            transform: 'rotateX(0deg)',
            pointerEvents: 'auto',
          },
          [`${keyframePercent(fadeOutSeconds, cycleSeconds)}%`]: {
            opacity: 0,
            transform: 'rotateX(100deg)',
            pointerEvents: 'none',
          },
          '100%': {opacity: 0, transform: 'rotateX(100deg)', pointerEvents: 'none'},
        },
        position: 'relative',
        width: CARD_WIDTH,
        height: CARD_HEIGHT,
        perspective: 500,
      }}
    >
      {templates.map((template, index) => (
        <Box
          key={template.value}
          component="button"
          type="button"
          onClick={() => onSelect(template.value)}
          sx={{
            position: 'absolute',
            inset: 0,
            display: 'flex',
            alignItems: 'center',
            gap: 1.5,
            px: 2,
            border: '1px solid',
            borderColor: isDark ? 'rgba(255,255,255,0.10)' : 'rgba(0,0,0,0.08)',
            borderRadius: 1.5,
            bgcolor: isDark ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.03)',
            cursor: 'pointer',
            font: 'inherit',
            backfaceVisibility: 'hidden',
            animation: `${animationName} ${cycleSeconds}s infinite backwards`,
            animationDelay: `${index * staggerSeconds}s`,
            '&:hover': {
              borderColor: 'primary.main',
            },
          }}
        >
          <Box
            sx={{
              width: ICON_SIZE,
              height: ICON_SIZE,
              display: 'flex',
              flexShrink: 0,
              '& svg': {width: '100%', height: '100%'},
            }}
          >
            {template.icon}
          </Box>
          <Typography variant="body1" color="text.primary" noWrap>
            {t(template.titleKey)}
          </Typography>
        </Box>
      ))}
    </Box>
  );
}

export interface FrameworkFlipCardProps {
  /** How many flip-card slots to show side by side (1 to SLOT_COUNT). Defaults to all of them. */
  slotCount?: number;
}

/** Cycles through the SDKs ThunderID ships, several at a time; clicking a card jumps straight to its application template. */
export default function FrameworkFlipCard({slotCount = SLOT_COUNT}: FrameworkFlipCardProps): JSX.Element {
  const {mode} = useColorScheme();
  const navigate = useNavigate();
  const isDark = mode === 'dark';

  const handleSelect = (templateId: string) => {
    navigate(`${RouteConfig.applications.types()}?type=${templateId}`)?.catch(() => undefined);
  };

  return (
    <Stack direction="row" spacing={1.5}>
      {SLOTS.slice(0, slotCount).map((templates, slotIndex) => (
        <FlipSlot
          // eslint-disable-next-line react/no-array-index-key -- slots are a fixed, stable partition of AVAILABLE_TEMPLATES
          key={slotIndex}
          templates={templates}
          slotIndex={slotIndex}
          staggerSeconds={SLOT_STAGGER_SECONDS[slotIndex % SLOT_STAGGER_SECONDS.length]}
          isDark={isDark}
          onSelect={handleSelect}
        />
      ))}
    </Stack>
  );
}
