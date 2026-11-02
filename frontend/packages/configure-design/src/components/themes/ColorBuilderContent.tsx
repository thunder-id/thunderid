// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-member-access */

import {Stack, type CssVarsPalette} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ColorEditRow from '../common/ColorEditRow';
import ConfigCard from '../common/ConfigCard';

export interface ColorBuilderContentProps {
  colors: CssVarsPalette;
  onUpdate: (updater: (c: CssVarsPalette) => void) => void;
}

/**
 * ColorBuilderContent - Theme builder section for editing color scheme colors.
 * Organizes colors into ConfigCard sections matching the full palette structure.
 * Channel-only fields (e.g. mainChannel) are intentionally omitted.
 */
export default function ColorBuilderContent({colors, onUpdate}: ColorBuilderContentProps): JSX.Element {
  const {t} = useTranslation('design');
  // Cast to any to allow dynamic string indexing on CssVarsPalette (no index signature).
  // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment
  const c = colors as any;

  const field = (path: string[], label: string): JSX.Element => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return
    const value: string = path.reduce((obj: any, key) => obj?.[key], c) ?? '';
    return (
      <ColorEditRow
        label={label}
        value={value}
        onChange={(v) => {
          onUpdate((palette) => {
            // eslint-disable-next-line @typescript-eslint/no-explicit-any, @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-return
            const target = path.slice(0, -1).reduce<any>((obj: any, key) => (obj[key] ??= {}), palette);

            target[path[path.length - 1]] = v;
          });
        }}
      />
    );
  };

  const main = t('themes.forms.color_builder.fields.main.label', 'Main');
  const light = t('themes.forms.color_builder.fields.light.label', 'Light');
  const dark = t('themes.forms.color_builder.fields.dark.label', 'Dark');
  const contrastText = t('themes.forms.color_builder.fields.contrast_text.label', 'Contrast text');

  return (
    <Stack gap={1}>
      {/* Primary */}
      <ConfigCard title={t('themes.forms.color_builder.primary.title', 'Primary')}>
        {field(['primary', 'main'], main)}
        {field(['primary', 'light'], light)}
        {field(['primary', 'dark'], dark)}
        {field(['primary', 'contrastText'], contrastText)}
      </ConfigCard>

      {/* Secondary */}
      <ConfigCard title={t('themes.forms.color_builder.secondary.title', 'Secondary')} defaultOpen={false}>
        {field(['secondary', 'main'], main)}
        {field(['secondary', 'light'], light)}
        {field(['secondary', 'dark'], dark)}
        {field(['secondary', 'contrastText'], contrastText)}
      </ConfigCard>

      {/* Semantic: Error */}
      {c.error && (
        <ConfigCard title={t('themes.forms.color_builder.error.title', 'Error')} defaultOpen={false}>
          {field(['error', 'main'], main)}
          {field(['error', 'light'], light)}
          {field(['error', 'dark'], dark)}
          {field(['error', 'contrastText'], contrastText)}
        </ConfigCard>
      )}

      {/* Semantic: Warning */}
      {c.warning && (
        <ConfigCard title={t('themes.forms.color_builder.warning.title', 'Warning')} defaultOpen={false}>
          {field(['warning', 'main'], main)}
          {field(['warning', 'light'], light)}
          {field(['warning', 'dark'], dark)}
          {field(['warning', 'contrastText'], contrastText)}
        </ConfigCard>
      )}

      {/* Semantic: Info */}
      {c.info && (
        <ConfigCard title={t('themes.forms.color_builder.info.title', 'Info')} defaultOpen={false}>
          {field(['info', 'main'], main)}
          {field(['info', 'light'], light)}
          {field(['info', 'dark'], dark)}
          {field(['info', 'contrastText'], contrastText)}
        </ConfigCard>
      )}

      {/* Semantic: Success */}
      {c.success && (
        <ConfigCard title={t('themes.forms.color_builder.success.title', 'Success')} defaultOpen={false}>
          {field(['success', 'main'], main)}
          {field(['success', 'light'], light)}
          {field(['success', 'dark'], dark)}
          {field(['success', 'contrastText'], contrastText)}
        </ConfigCard>
      )}

      {/* Backgrounds */}
      {c.background && (
        <ConfigCard title={t('themes.forms.color_builder.backgrounds.title', 'Backgrounds')} defaultOpen={false}>
          {field(['background', 'default'], t('themes.forms.color_builder.fields.default.label', 'Default'))}
          {field(['background', 'paper'], t('themes.forms.color_builder.fields.surface.label', 'Surface'))}
          {c.background.acrylic !== undefined &&
            field(['background', 'acrylic'], t('themes.forms.color_builder.fields.acrylic.label', 'Acrylic'))}
        </ConfigCard>
      )}

      {/* Text */}
      {c.text && (
        <ConfigCard title={t('themes.forms.color_builder.text.title', 'Text')} defaultOpen={false}>
          {field(['text', 'primary'], t('themes.forms.color_builder.fields.primary.label', 'Primary'))}
          {field(['text', 'secondary'], t('themes.forms.color_builder.fields.secondary.label', 'Secondary'))}
          {c.text.disabled !== undefined &&
            field(['text', 'disabled'], t('themes.forms.color_builder.fields.disabled.label', 'Disabled'))}
        </ConfigCard>
      )}

      {/* Common */}
      {c.common && (
        <ConfigCard title={t('themes.forms.color_builder.common.title', 'Common')} defaultOpen={false}>
          {field(['common', 'black'], t('themes.forms.color_builder.fields.black.label', 'Black'))}
          {field(['common', 'white'], t('themes.forms.color_builder.fields.white.label', 'White'))}
          {c.common.background !== undefined &&
            field(['common', 'background'], t('themes.forms.color_builder.fields.background.label', 'Background'))}
          {c.common.onBackground !== undefined &&
            field(
              ['common', 'onBackground'],
              t('themes.forms.color_builder.fields.on_background.label', 'On background'),
            )}
        </ConfigCard>
      )}

      {/* Borders & Dividers */}
      {c.divider !== undefined && (
        <ConfigCard title={t('themes.forms.color_builder.borders.title', 'Borders & dividers')} defaultOpen={false}>
          {field(['divider'], t('themes.forms.color_builder.fields.divider.label', 'Divider'))}
        </ConfigCard>
      )}
    </Stack>
  );
}
