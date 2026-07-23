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

import {generateAvatarDataUri, type AvatarParams} from '@thunderid/react';
import {Box, Button, Stack} from '@wso2/oxygen-ui';
import {Shuffle} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {sampleIndices, sampleIndicesIncluding} from './utils/sampleIndices';

const DEFAULT_SAMPLE_SIZE = 8;

/**
 * Props for the {@link AvatarSwatchGrid} component.
 *
 * @public
 */
export interface AvatarSwatchGridProps {
  /**
   * Fixed avatar params for this grid — every swatch shares the same shape, variant, and
   * content, only `colors` (background gradient rotation) varies between tiles.
   */
  base: Omit<AvatarParams, 'bg' | 'colors'>;

  /**
   * The currently selected gradient rotation index.
   */
  value: number;

  /**
   * Total number of distinct gradient rotations available to sample from (see
   * `AVATAR_GRADIENT_COUNT`).
   */
  gradientCount: number;

  /**
   * Number of swatches to show at once (a random sample re-drawn on shuffle).
   *
   * @defaultValue 8
   */
  optionCount?: number;

  /**
   * Whether to render the grid's own "Shuffle" button. Set to `false` when an ancestor already
   * renders a shuffle trigger and drives reshuffling via `shuffleSignal`.
   *
   * @defaultValue true
   */
  showShuffle?: boolean;

  /**
   * Bump this (e.g. an incrementing counter) to trigger a reshuffle from an ancestor's own
   * shuffle button when `showShuffle` is `false`.
   */
  shuffleSignal?: number;

  /**
   * Fired when the user picks a swatch, with its gradient rotation index.
   */
  onChange: (colors: number) => void;
}

/**
 * A shuffled grid of avatar swatches that all share the same shape/variant/content and only
 * differ in background gradient, so the user can pick a background at a glance instead of
 * fiddling with a color input. Contains no dialog chrome, embed this inside a dialog or any
 * other container.
 *
 * @public
 */
export default function AvatarSwatchGrid({
  base,
  value,
  gradientCount,
  optionCount = DEFAULT_SAMPLE_SIZE,
  showShuffle = true,
  shuffleSignal = undefined,
  onChange,
}: AvatarSwatchGridProps): JSX.Element {
  const {t} = useTranslation('elements');
  const sampleSize: number = Math.min(optionCount, gradientCount);
  const [visible, setVisible] = useState<number[]>((): number[] =>
    sampleIndicesIncluding(gradientCount, sampleSize, value),
  );
  const [prevSignal, setPrevSignal] = useState<number | undefined>(shuffleSignal);

  if (shuffleSignal !== undefined && shuffleSignal !== prevSignal) {
    setPrevSignal(shuffleSignal);
    setVisible(sampleIndices(gradientCount, sampleSize));
  }

  const handleShuffle = useCallback((): void => {
    setVisible(sampleIndices(gradientCount, sampleSize));
  }, [gradientCount, sampleSize]);

  return (
    <Stack spacing={1.5} sx={{p: 1.5}}>
      {showShuffle && (
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button size="small" startIcon={<Shuffle size={14} />} onClick={handleShuffle}>
            {t('avatar_swatch_grid.shuffle', 'Shuffle')}
          </Button>
        </Box>
      )}
      <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
        {visible.map((colors) => (
          <Box
            key={colors}
            component="button"
            type="button"
            aria-label={t('avatar_swatch_grid.swatch', 'Background option')}
            onClick={() => onChange(colors)}
            sx={{
              width: 42,
              height: 42,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              p: 0,
              border: '2px solid',
              borderColor: value === colors ? 'primary.main' : 'transparent',
              borderRadius: base.shape === 'circle' ? '50%' : '22%',
              overflow: 'hidden',
              cursor: 'pointer',
              transition: 'all 0.1s',
              '&:hover': {borderColor: 'primary.light'},
            }}
          >
            <img
              src={generateAvatarDataUri({...base, colors})}
              alt=""
              style={{width: '100%', height: '100%', objectFit: 'cover'}}
            />
          </Box>
        ))}
      </Box>
    </Stack>
  );
}
