// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useBaseUrl from '@docusaurus/useBaseUrl';
import {Box} from '@wso2/oxygen-ui';
import {useEffect, useState, type JSX} from 'react';
import type {CommunityDaySkyline} from '@site/src/components/CommunityDay';
import {ACCENTS, useHomeTokens} from '@site/src/components/HomePage/homeTheme';

// Fades out the bottom of the drawing through its water, so the skyline dissolves into the background.
const BOTTOM_FADE = 'linear-gradient(to bottom, #000 95%, transparent 100%)';
// Where the landmark sits across the hero on wide screens, as a percentage: left of the title, clear of the logo.
const WIDE_LANDMARK_LEFT = 26;
// Clears a soft-edged rectangle behind the hero text on wide screens, from the partner logo down to the summary, so
// the text sits on a calm background. The two layers add up: the drawing shows outside the text's width or outside
// its height. Sized in pixels, so it matches the text column at every screen width.
const TEXT_CLEARING = [
  'linear-gradient(to right, #000 calc(50% - 400px), transparent calc(50% - 290px), transparent calc(50% + 290px), #000 calc(50% + 400px))',
  'linear-gradient(to bottom, #000 70px, transparent 130px, transparent 540px, #000 600px)',
].join(', ');

/**
 * The event city's skyline drawn in brand-colored lines across the top of the hero, behind the logo and title, with
 * its main landmark beside the title. The drawing holds only the line shapes as transparency, so the lines take their
 * color from the background below and follow the theme.
 */
export default function HeroSkyline({skyline}: {skyline: CommunityDaySkyline}): JSX.Element {
  const t = useHomeTokens();
  const src = useBaseUrl(skyline.src);
  const skylineUrl = `url(${src})`;
  // Until the drawing loads, its color layers would show as solid shapes, so the skyline stays hidden until then.
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const image = new Image();
    image.onload = () => setLoaded(true);
    image.src = src;
  }, [src]);
  const landmarkPercent = skyline.landmarkX * 100;
  // Wide enough that the drawing still reaches both edges of the hero once the landmark is moved left of the title.
  const wideWidth = `${Math.ceil(
    Math.max(100, WIDE_LANDMARK_LEFT / skyline.landmarkX, (100 - WIDE_LANDMARK_LEFT) / (1 - skyline.landmarkX)),
  )}%`;
  const mask = `${skylineUrl}, ${BOTTOM_FADE}`;

  return (
    <Box
      aria-hidden
      sx={{
        position: 'absolute',
        inset: 0,
        overflow: 'hidden',
        pointerEvents: 'none',
        opacity: loaded ? 1 : 0,
        transition: 'opacity 0.6s ease',
        maskImage: {lg: TEXT_CLEARING},
        WebkitMaskImage: {lg: TEXT_CLEARING},
      }}
    >
      <Box
        sx={{
          position: 'absolute',
          top: {xs: 120, md: 60},
          left: {xs: '50%', lg: `${WIDE_LANDMARK_LEFT}%`},
          transform: `translateX(-${landmarkPercent}%)`,
          width: {xs: '260%', md: '140%', lg: wideWidth},
          aspectRatio: skyline.aspectRatio,
          opacity: t.isDark ? 0.32 : 0.26,
          background: t.isDark
            ? `linear-gradient(to right, ${ACCENTS.cyan}, ${ACCENTS.electric} 50%, ${ACCENTS.violet})`
            : `linear-gradient(to right, ${ACCENTS.electric}, ${ACCENTS.violet})`,
          maskImage: mask,
          WebkitMaskImage: mask,
          maskSize: '100% 100%',
          WebkitMaskSize: '100% 100%',
          maskRepeat: 'no-repeat',
          WebkitMaskRepeat: 'no-repeat',
          maskComposite: 'intersect',
          WebkitMaskComposite: 'source-in',
        }}
      />
    </Box>
  );
}
