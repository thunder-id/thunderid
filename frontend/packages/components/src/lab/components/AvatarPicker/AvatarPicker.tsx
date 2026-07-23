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

import {
  generateAvatarDataUri,
  deriveAvatarContent,
  ANONYMOUS_ANIMAL_ICONS,
  ANONYMOUS_ENTITY_ICONS,
  type AvatarParams,
  type AvatarShape,
  type AvatarVariant,
} from '@thunderid/react';
import {Box, Button, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Shuffle} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import IconGridPicker from '../IconGridPicker/IconGridPicker';

const SHAPES: {label: string; value: AvatarShape}[] = [
  {label: 'Rounded', value: 'rounded'},
  {label: 'Circle', value: 'circle'},
];

const CONTENT_VARIANTS: {label: string; value: AvatarVariant}[] = [
  {label: 'Blank', value: 'blank'},
  {label: 'Two letters', value: 'two_letter'},
  {label: 'One letter', value: 'one_letter'},
];

/**
 * Props for the {@link AvatarPicker} component.
 *
 * @public
 */
export interface AvatarPickerProps {
  /**
   * The current avatar parameters.
   */
  value: AvatarParams;

  /**
   * Fired whenever the shape, variant, content, colors, or background changes.
   */
  onChange: (params: AvatarParams) => void;
}

/**
 * A panel for building an `avatar:` spec: a background shape (rounded/circle), a content
 * variant (two letters, one letter, or a curated anonymous animal), an optional background
 * color override, and (for letter variants) a shuffleable color and seed-text-derived
 * initials. Contains no dialog chrome, embed this inside a dialog or any other container.
 *
 * @public
 */
export default function AvatarPicker({value, onChange}: AvatarPickerProps): JSX.Element {
  const {t} = useTranslation('elements');
  const [seedText, setSeedText] = useState<string>(value.content);
  const isAnimal: boolean = value.variant === 'anonymous_animal';
  const isEntity: boolean = value.variant === 'anonymous_entity';
  const isBlank: boolean = value.variant === 'blank';
  const isLetterVariant: boolean = value.variant === 'one_letter' || value.variant === 'two_letter';
  const contentTypeLabel: string = isLetterVariant
    ? t('avatar_picker.content_type.text_avatar', 'Text Avatar')
    : t('avatar_picker.content_type.avatar', 'Avatar');

  const handleSeedChange = useCallback(
    (text: string): void => {
      setSeedText(text);
      onChange({...value, content: deriveAvatarContent(value.variant, text)});
    },
    [value, onChange],
  );

  const handleVariantChange = useCallback(
    (variant: AvatarVariant): void => {
      const content: string =
        variant === 'anonymous_animal' || variant === 'anonymous_entity'
          ? value.content
          : deriveAvatarContent(variant, seedText);
      onChange({...value, variant, content});
    },
    [value, seedText, onChange],
  );

  const handleShuffleColors = useCallback((): void => {
    onChange({...value, colors: value.colors + 1});
  }, [value, onChange]);

  const animalIcons: Record<string, string> = useMemo(
    () =>
      isAnimal
        ? Object.fromEntries(
            Object.keys(ANONYMOUS_ANIMAL_ICONS).map((name) => [
              name,
              generateAvatarDataUri({...value, variant: 'anonymous_animal', content: name}),
            ]),
          )
        : {},
    [isAnimal, value],
  );

  const entityIcons: Record<string, string> = useMemo(
    () =>
      isEntity
        ? Object.fromEntries(
            Object.keys(ANONYMOUS_ENTITY_ICONS).map((name) => [
              name,
              generateAvatarDataUri({...value, variant: 'anonymous_entity', content: name}),
            ]),
          )
        : {},
    [isEntity, value],
  );

  return (
    <Stack spacing={1.5} sx={{p: 1.5}}>
      <Box sx={{display: 'flex', gap: 1}}>
        {SHAPES.map(({label, value: shapeValue}) => {
          const buttonLabel: string =
            SHAPES.length > 1
              ? `${t(`avatar_picker.shapes.${shapeValue}`, label)} ${contentTypeLabel}`
              : contentTypeLabel;

          return (
            <Button
              key={shapeValue}
              size="small"
              variant={value.shape === shapeValue ? 'contained' : 'outlined'}
              onClick={() => onChange({...value, shape: shapeValue})}
            >
              {buttonLabel}
            </Button>
          );
        })}
      </Box>

      <Box sx={{display: 'flex', gap: 1, flexWrap: 'wrap'}}>
        {CONTENT_VARIANTS.map(({label, value: variantValue}) => (
          <Button
            key={variantValue}
            size="small"
            variant={value.variant === variantValue ? 'contained' : 'outlined'}
            onClick={() => handleVariantChange(variantValue)}
          >
            {t(`avatar_picker.variants.${variantValue}`, label)}
          </Button>
        ))}
        <Button
          size="small"
          variant={isAnimal ? 'contained' : 'outlined'}
          onClick={() => handleVariantChange('anonymous_animal')}
        >
          {t('avatar_picker.variants.anonymous_animal', 'Anonymous animal')}
        </Button>
        <Button
          size="small"
          variant={isEntity ? 'contained' : 'outlined'}
          onClick={() => handleVariantChange('anonymous_entity')}
        >
          {t('avatar_picker.variants.anonymous_entity', 'Entity')}
        </Button>
      </Box>

      <Stack direction="row" spacing={1} alignItems="center">
        <Typography variant="caption" color="text.secondary">
          {t('avatar_picker.background.label', 'Background color')}
        </Typography>
        <Box
          component="input"
          type="color"
          value={value.bg ?? '#888888'}
          onChange={(e) => onChange({...value, bg: e.target.value})}
          sx={{width: 28, height: 28, border: 'none', p: 0, background: 'none', cursor: 'pointer'}}
        />
        {value.bg && (
          <Button size="small" onClick={() => onChange({...value, bg: undefined})}>
            {t('avatar_picker.background.reset', 'Auto')}
          </Button>
        )}
      </Stack>

      {isAnimal ? (
        <IconGridPicker
          icons={animalIcons}
          value={value.content}
          shape={value.shape}
          onChange={(name) => onChange({...value, content: name})}
        />
      ) : isEntity ? (
        <IconGridPicker
          icons={entityIcons}
          value={value.content}
          shape={value.shape}
          onChange={(name) => onChange({...value, content: name})}
        />
      ) : (
        <>
          {!isBlank && (
            <TextField
              fullWidth
              size="small"
              label={t('avatar_picker.seed_text.label', 'Seed text')}
              placeholder={t('avatar_picker.seed_text.placeholder', 'e.g. your app name')}
              value={seedText}
              onChange={(e) => handleSeedChange(e.target.value)}
            />
          )}
          <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Box
              component="img"
              src={generateAvatarDataUri(value)}
              alt=""
              sx={{width: 52, height: 52, borderRadius: value.shape === 'circle' ? '50%' : '22%'}}
            />
            <Button size="small" startIcon={<Shuffle size={14} />} onClick={handleShuffleColors}>
              {t('avatar_picker.shuffle', 'Shuffle colors')}
            </Button>
          </Box>
        </>
      )}
    </Stack>
  );
}
