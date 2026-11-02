// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Theme} from '@thunderid/design';
import {Box, Slider, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ConfigCard from '../common/ConfigCard';
import SelectRow from '../common/SelectRow';

export interface ShapeBuilderContentProps {
  draft: Theme;
  onUpdate: (updater: (d: Theme) => void) => void;
}

/**
 * ShapeBuilderContent - Theme builder section for editing border radius.
 * Provides slider control and preset swatches for quick selection.
 */
export default function ShapeBuilderContent({draft, onUpdate}: ShapeBuilderContentProps): JSX.Element {
  const {t} = useTranslation('design');
  const borderRadiusNum =
    typeof draft.shape?.borderRadius === 'number'
      ? draft.shape.borderRadius
      : parseInt(String(draft.shape?.borderRadius ?? '8'), 10) || 8;

  const PRESETS = [0, 4, 8, 12, 16, 24];

  const draftRecord = draft;
  const borderWidth: string = (draftRecord.border?.width as string | undefined) ?? '1px';
  const borderStyle: string = (draftRecord.border?.style as string | undefined) ?? 'solid';

  return (
    <Stack gap={1}>
      <ConfigCard title={t('themes.forms.shape_builder.border_radius.title', 'Border radius')}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" mb={0.5}>
          <Typography variant="body2" sx={{fontWeight: 600, fontSize: '0.875rem', color: 'text.secondary'}}>
            {t('themes.forms.shape_builder.fields.radius.label', 'Radius')}
          </Typography>
          <Box
            sx={{
              px: 1.25,
              py: 0.25,
              border: '1.5px solid',
              borderColor: 'divider',
              borderRadius: 1.5,
              minWidth: 52,
              textAlign: 'center',
            }}
          >
            <Typography sx={{fontFamily: 'monospace', fontSize: '0.8125rem', fontWeight: 600}}>
              {borderRadiusNum}px
            </Typography>
          </Box>
        </Stack>

        <Slider
          size="small"
          min={0}
          max={24}
          step={1}
          value={borderRadiusNum}
          onChange={(_, v) =>
            onUpdate((d) => {
              if (d.shape) Object.assign(d.shape, {borderRadius: v});
            })
          }
          sx={{py: 1}}
        />

        {/* Preset swatches */}
        <Stack direction="row" spacing={1} sx={{mt: 0.5, flexWrap: 'wrap', gap: 0.75}}>
          {PRESETS.map((r) => (
            <Box
              key={r}
              onClick={() =>
                onUpdate((d) => {
                  if (d.shape) Object.assign(d.shape, {borderRadius: r});
                })
              }
              sx={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'center',
                gap: 0.5,
                cursor: 'pointer',
              }}
            >
              <Box
                sx={{
                  width: 32,
                  height: 32,
                  bgcolor: borderRadiusNum === r ? 'primary.main' : 'action.selected',
                  borderRadius: `${r}px`,
                  border: '1.5px solid',
                  borderColor: borderRadiusNum === r ? 'primary.main' : 'transparent',
                  transition: 'all 0.15s',
                  '&:hover': {bgcolor: borderRadiusNum === r ? 'primary.dark' : 'action.hover'},
                }}
              />
              <Typography
                variant="caption"
                sx={{
                  fontSize: '0.6rem',
                  color: borderRadiusNum === r ? 'primary.main' : 'text.disabled',
                  fontWeight: borderRadiusNum === r ? 700 : 400,
                }}
              >
                {r}
              </Typography>
            </Box>
          ))}
        </Stack>
      </ConfigCard>

      <ConfigCard title={t('themes.forms.shape_builder.border_style.title', 'Border style')} defaultOpen={false}>
        <SelectRow
          label={t('themes.forms.shape_builder.fields.width.label', 'Width')}
          value={borderWidth}
          options={[
            {value: '0px', label: t('themes.forms.shape_builder.fields.width.options.none.label', 'None (0px)')},
            {value: '1px', label: t('themes.forms.shape_builder.fields.width.options.thin.label', 'Thin (1px)')},
            {value: '2px', label: t('themes.forms.shape_builder.fields.width.options.medium.label', 'Medium (2px)')},
            {value: '3px', label: t('themes.forms.shape_builder.fields.width.options.thick.label', 'Thick (3px)')},
          ]}
          onChange={(v) =>
            onUpdate((d) => {
              const dr = d;
              dr.border = {...(dr.border ?? {}), width: v};
            })
          }
        />
        <SelectRow
          label={t('themes.forms.shape_builder.fields.style.label', 'Style')}
          value={borderStyle}
          options={[
            {value: 'solid', label: t('themes.forms.shape_builder.fields.style.options.solid.label', 'Solid')},
            {value: 'dashed', label: t('themes.forms.shape_builder.fields.style.options.dashed.label', 'Dashed')},
            {value: 'dotted', label: t('themes.forms.shape_builder.fields.style.options.dotted.label', 'Dotted')},
            {value: 'none', label: t('themes.forms.shape_builder.fields.style.options.none.label', 'None')},
          ]}
          onChange={(v) =>
            onUpdate((d) => {
              const dr = d;
              dr.border = {...(dr.border ?? {}), style: v};
            })
          }
        />
      </ConfigCard>
    </Stack>
  );
}
