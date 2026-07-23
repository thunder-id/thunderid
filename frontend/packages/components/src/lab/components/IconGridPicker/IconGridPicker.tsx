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

import type {AvatarShape} from '@thunderid/react';
import {Box, Button, Stack} from '@wso2/oxygen-ui';
import {Shuffle} from '@wso2/oxygen-ui-icons-react';
import {useState, useMemo, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {sampleIncluding} from './utils/sample';

const DEFAULT_SAMPLE_SIZE = 8;

/**
 * Props for the {@link IconGridPicker} component.
 *
 * @public
 */
export interface IconGridPickerProps {
  /**
   * Map of icon name to its `svg+xml` data URI.
   */
  icons: Record<string, string>;

  /**
   * The currently selected icon name (unprefixed), if any.
   */
  value?: string;

  /**
   * The shape all icons in this grid are rendered with, used to match the swatch's own
   * clipping to the icon's shape (a circle icon inside a rounded-square swatch would leave
   * its corners showing the swatch background).
   */
  shape: AvatarShape;

  /**
   * Number of icons to show at once (a random sample re-drawn on shuffle).
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
   * Fired when the user picks an icon, with its (unprefixed) name.
   */
  onChange: (name: string) => void;
}

/**
 * A shuffled grid of curated icon options with a reshuffle button.
 * Contains no dialog chrome — embed this inside a dialog or any other container.
 *
 * @public
 */
export default function IconGridPicker({
  icons,
  value = '',
  shape,
  optionCount = DEFAULT_SAMPLE_SIZE,
  showShuffle = true,
  shuffleSignal = undefined,
  onChange,
}: IconGridPickerProps): JSX.Element {
  const {t} = useTranslation('elements');
  const allNames: string[] = useMemo((): string[] => Object.keys(icons), [icons]);
  const [visibleNames, setVisibleNames] = useState<string[]>((): string[] =>
    sampleIncluding(allNames, optionCount, value),
  );
  const [prevSignal, setPrevSignal] = useState<number | undefined>(shuffleSignal);

  if (shuffleSignal !== undefined && shuffleSignal !== prevSignal) {
    setPrevSignal(shuffleSignal);
    setVisibleNames(sampleIncluding(allNames, optionCount, value));
  }

  const handleShuffle = useCallback((): void => {
    setVisibleNames(sampleIncluding(allNames, optionCount, value));
  }, [allNames, optionCount, value]);

  return (
    <Stack spacing={1.5} sx={{p: 1.5}}>
      {showShuffle && (
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button size="small" startIcon={<Shuffle size={14} />} onClick={handleShuffle}>
            {t('icon_grid_picker.shuffle', 'Shuffle')}
          </Button>
        </Box>
      )}
      <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
        {visibleNames.map((name) => (
          <Box
            key={name}
            component="button"
            type="button"
            onClick={() => onChange(name)}
            title={name}
            sx={{
              width: 42,
              height: 42,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              p: 0,
              border: '2px solid',
              borderColor: value === name ? 'primary.main' : 'transparent',
              bgcolor: value === name ? 'primary.light' : 'action.hover',
              borderRadius: shape === 'circle' ? '50%' : '22%',
              overflow: 'hidden',
              cursor: 'pointer',
              transition: 'all 0.1s',
              '&:hover': {borderColor: 'primary.light'},
            }}
          >
            <img src={icons[name]} alt={name} style={{width: '100%', height: '100%', objectFit: 'cover'}} />
          </Box>
        ))}
      </Box>
    </Stack>
  );
}
