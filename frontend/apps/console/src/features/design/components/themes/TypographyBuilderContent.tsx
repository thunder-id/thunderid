// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CspOriginHint} from '@thunderid/components';
import {
  BROWSER_SAFE_FONTS,
  DEFAULT_FONT_STACK,
  getFontImportURL,
  useFontStylesheetLink,
  type Theme,
} from '@thunderid/design';
import {
  Autocomplete,
  Box,
  FormControl,
  FormLabel,
  Link,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import ConfigCard from '../common/ConfigCard';
import SelectRow from '../common/SelectRow';
import SliderRow from '../common/SliderRow';

const FONT_WEIGHT_OPTIONS = [
  {value: '100', label: '100 — Thin'},
  {value: '200', label: '200 — Extra Light'},
  {value: '300', label: '300 — Light'},
  {value: '400', label: '400 — Regular'},
  {value: '500', label: '500 — Medium'},
  {value: '600', label: '600 — Semi Bold'},
  {value: '700', label: '700 — Bold'},
  {value: '800', label: '800 — Extra Bold'},
  {value: '900', label: '900 — Black'},
];

/** MUI's mapping: which fontWeight reference each typography variant uses by default. */
const VARIANT_WEIGHT_REF: Record<
  string,
  'fontWeightLight' | 'fontWeightRegular' | 'fontWeightMedium' | 'fontWeightBold'
> = {
  h1: 'fontWeightLight',
  h2: 'fontWeightLight',
  h3: 'fontWeightRegular',
  h4: 'fontWeightRegular',
  h5: 'fontWeightRegular',
  h6: 'fontWeightMedium',
  subtitle1: 'fontWeightRegular',
  subtitle2: 'fontWeightMedium',
  body1: 'fontWeightRegular',
  body2: 'fontWeightRegular',
  button: 'fontWeightMedium',
  caption: 'fontWeightRegular',
  overline: 'fontWeightRegular',
};

type TypographyRecord = Record<string, unknown>;

/** Propagate a fontWeight reference change to all variants that use it. */
function propagateWeight(typography: TypographyRecord, weightKey: string, value: number): void {
  for (const [variant, ref] of Object.entries(VARIANT_WEIGHT_REF)) {
    if (ref === weightKey && typography[variant]) {
      (typography[variant] as Record<string, unknown>).fontWeight = value;
    }
  }
}

/** Default font weight values. */
const DEFAULT_WEIGHTS = {fontWeightLight: 300, fontWeightRegular: 400, fontWeightMedium: 500, fontWeightBold: 700};

/** Default base size values. */
const DEFAULT_SIZES = {fontSize: 14, htmlFontSize: 16};

/** All typography variant keys. */
const VARIANT_KEYS = [
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'subtitle1',
  'subtitle2',
  'body1',
  'body2',
  'button',
  'caption',
  'overline',
];

/** Clear computed variant properties so extendTheme recomputes them from the base values.
 *  Removes fontSize, lineHeight, and letterSpacing from all variants. */
function clearVariantSizes(typography: TypographyRecord): void {
  for (const key of VARIANT_KEYS) {
    const variantObj = typography[key] as Record<string, unknown> | undefined;
    if (!variantObj) continue;
    delete variantObj.fontSize;
    delete variantObj.lineHeight;
    delete variantObj.letterSpacing;
  }
}

const TYPE_SCALE_VARIANTS: {key: string; label: string}[] = [
  {key: 'h1', label: 'h1'},
  {key: 'h2', label: 'h2'},
  {key: 'h3', label: 'h3'},
  {key: 'h4', label: 'h4'},
  {key: 'h5', label: 'h5'},
  {key: 'h6', label: 'h6'},
  {key: 'subtitle1', label: 'subtitle1'},
  {key: 'subtitle2', label: 'subtitle2'},
  {key: 'body1', label: 'body1'},
  {key: 'body2', label: 'body2'},
  {key: 'button', label: 'button'},
  {key: 'caption', label: 'caption'},
  {key: 'overline', label: 'overline'},
];

export interface TypographyBuilderContentProps {
  draft: Theme;
  onUpdate: (updater: (d: Theme) => void) => void;
}

/**
 * TypographyBuilderContent - Theme builder section for font family configuration.
 * Provides an autocomplete to select from a fixed list of browser-safe fonts.
 */
export default function TypographyBuilderContent({draft, onUpdate}: TypographyBuilderContentProps): JSX.Element {
  const {t} = useTranslation('design');
  const fontFamily = (draft.typography?.fontFamily as string) ?? '';
  const importURL = getFontImportURL(draft) ?? '';
  const typo = draft.typography;

  useFontStylesheetLink(importURL || undefined);

  const [fontMode, setFontMode] = useState<'web-safe' | 'import'>(importURL ? 'import' : 'web-safe');

  const [stashedWebSafeFamily, setStashedWebSafeFamily] = useState(importURL ? '' : fontFamily);
  const [stashedImport, setStashedImport] = useState<{family: string; url: string}>(
    importURL ? {family: fontFamily, url: importURL} : {family: '', url: ''},
  );

  // Re-sync the toggle and stashed values when the draft is reverted or changed from outside.
  const [prevImportURL, setPrevImportURL] = useState(importURL);
  const [prevFontFamily, setPrevFontFamily] = useState(fontFamily);
  if (importURL !== prevImportURL || fontFamily !== prevFontFamily) {
    setPrevImportURL(importURL);
    setPrevFontFamily(fontFamily);
    if (importURL) {
      setFontMode('import');
      setStashedImport({family: fontFamily, url: importURL});
    } else {
      setFontMode('web-safe');
      setStashedWebSafeFamily(fontFamily);
      setStashedImport({family: '', url: ''});
    }
  }

  const applyFont = (family: string, url: string): void => {
    onUpdate((d) => {
      if (!d.typography) return;
      Object.assign(d.typography, {fontFamily: family});
      const typography = d.typography as unknown as {font?: {importURL?: string}};
      if (url) {
        typography.font = {...(typography.font ?? {}), importURL: url};
      } else if (typography.font) {
        delete typography.font.importURL;
        if (Object.keys(typography.font).length === 0) delete typography.font;
      }
    });
  };

  const handleFontModeChange = (_: SyntheticEvent, value: 'web-safe' | 'import' | null): void => {
    if (!value || value === fontMode) return;
    let nextFamily: string;
    let nextURL: string;
    if (value === 'web-safe') {
      // Leaving import mode: remember its values and restore.
      setStashedImport({family: fontFamily, url: importURL});
      nextFamily = stashedWebSafeFamily;
      nextURL = '';
    } else {
      // Leaving web-safe mode: remember its font and restore.
      setStashedWebSafeFamily(fontFamily);
      nextFamily = stashedImport.family;
      nextURL = stashedImport.url;
    }
    applyFont(nextFamily, nextURL);
    // Pre-sync so the re-sync doesn't flip the mode with this intentional toggle.
    setPrevFontFamily(nextFamily);
    setPrevImportURL(nextURL);
    setFontMode(value);
  };

  const handleChange = (_: SyntheticEvent, value: string): void => {
    applyFont(value, '');
  };

  return (
    <Stack gap={1}>
      <ConfigCard title={t('themes.forms.typography_builder.font_family.title', 'Font Family')}>
        <ToggleButtonGroup
          value={fontMode}
          exclusive
          size="small"
          onChange={handleFontModeChange}
          fullWidth
          sx={{mb: 1.5}}
        >
          <ToggleButton value="web-safe" sx={{textTransform: 'none', fontSize: '0.75rem'}}>
            {t('themes.forms.typography_builder.font_family.modes.web_safe', 'Use a web-safe font')}
          </ToggleButton>
          <ToggleButton value="import" sx={{textTransform: 'none', fontSize: '0.75rem'}}>
            {t('themes.forms.typography_builder.font_family.modes.import', 'Use a Custom Font')}
          </ToggleButton>
        </ToggleButtonGroup>

        {fontMode === 'web-safe' ? (
          <Autocomplete
            disableClearable
            options={BROWSER_SAFE_FONTS}
            // The seeded default themes store the full fallback stack rather than the bare name.
            value={fontFamily === DEFAULT_FONT_STACK ? 'Inter Variable' : fontFamily || undefined}
            onChange={handleChange}
            renderOption={(props, option: string) => (
              <Box component="li" {...props} key={option}>
                <Typography sx={{fontFamily: option, fontSize: '0.875rem'}}>{option}</Typography>
              </Box>
            )}
            renderInput={(params: AutocompleteRenderInputParams) => (
              <TextField
                {...params}
                size="small"
                placeholder={t('themes.forms.typography_builder.fields.font_family.placeholder', 'Select a font')}
                helperText={t(
                  'themes.forms.typography_builder.fields.font_family.helper_text',
                  'Choose from the available web-safe fonts',
                )}
              />
            )}
            sx={{mb: 1.5}}
          />
        ) : (
          <Stack gap={1.5} sx={{mb: 1.5}}>
            <FormControl fullWidth>
              <FormLabel htmlFor="font-import-url-input">
                {t('themes.forms.typography_builder.fields.font_import_url.label', 'Font Import URL')}
              </FormLabel>
              <TextField
                fullWidth
                id="font-import-url-input"
                size="small"
                value={importURL}
                onChange={(e) => {
                  const newUrl = e.target.value;
                  const newFamily = newUrl ? fontFamily : '';
                  applyFont(newFamily, newUrl);
                  // Pre-sync so the re-sync effect doesn't flip mode when clearing the field.
                  setPrevFontFamily(newFamily);
                  setPrevImportURL(newUrl);
                }}
                placeholder={t(
                  'themes.forms.typography_builder.fields.font_import_url.placeholder',
                  'E.g., https://fonts.googleapis.com/css2?family=Poppins',
                )}
                helperText={t(
                  'themes.forms.typography_builder.fields.font_import_url.helper_text',
                  'Enter a URL to import a custom font from a font service.',
                )}
              />
            </FormControl>
            <CspOriginHint value={importURL} resourceType="font" />
            <FormControl fullWidth>
              <FormLabel htmlFor="font-family-input">
                {t('themes.forms.typography_builder.fields.font_family_input.label', 'Font Family')}
              </FormLabel>
              <TextField
                fullWidth
                id="font-family-input"
                size="small"
                value={fontFamily}
                onChange={(e) => {
                  const newFamily = e.target.value;
                  applyFont(newFamily, importURL);
                  // Pre-sync so the re-sync effect doesn't flip mode while typing a family name.
                  setPrevFontFamily(newFamily);
                  setPrevImportURL(importURL);
                }}
                placeholder={t('themes.forms.typography_builder.fields.font_family_input.placeholder', 'E.g. Poppins')}
                helperText={t(
                  'themes.forms.typography_builder.fields.font_family_input.helper_text',
                  'Enter the font family name documented by the font service above.',
                )}
              />
            </FormControl>
          </Stack>
        )}

        {/* Live preview of the selected font */}
        {fontFamily && (
          <Box
            sx={{
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: 1.5,
              p: 1.5,
              bgcolor: 'action.hover',
            }}
          >
            <Typography variant="caption" color="text.disabled" sx={{display: 'block', mb: 0.5, fontSize: '0.65rem'}}>
              {t('themes.forms.typography_builder.fields.preview.label', 'Preview')}
            </Typography>
            <Typography sx={{fontFamily: `${fontFamily}, ${DEFAULT_FONT_STACK}`, fontSize: '1rem', lineHeight: 1.4}}>
              The quick brown fox jumps over the lazy dog.
            </Typography>
            <Typography
              sx={{
                fontFamily: `${fontFamily}, ${DEFAULT_FONT_STACK}`,
                fontSize: '0.75rem',
                color: 'text.secondary',
                mt: 0.5,
              }}
            >
              ABCDEFGHIJKLMNOPQRSTUVWXYZ · 0123456789
            </Typography>
          </Box>
        )}
      </ConfigCard>

      <ConfigCard
        title={t('themes.forms.typography_builder.font_weights.title', 'Font Weights')}
        defaultOpen={false}
        action={
          <Link
            component="button"
            variant="caption"
            onClick={() => {
              onUpdate((d) => {
                if (!d.typography) return;
                Object.assign(d.typography, DEFAULT_WEIGHTS);
                for (const [wKey, wVal] of Object.entries(DEFAULT_WEIGHTS)) {
                  propagateWeight(d.typography as unknown as TypographyRecord, wKey, wVal);
                }
              });
            }}
            sx={{fontSize: '0.7rem', mr: 1, cursor: 'pointer', background: 'none', border: 'none', p: 0}}
          >
            {t('themes.forms.typography_builder.actions.reset.label', 'Reset')}
          </Link>
        }
      >
        <SelectRow
          label={t('themes.forms.typography_builder.fields.light.label', 'Light')}
          value={String((typo?.fontWeightLight as number | undefined) ?? 300)}
          options={FONT_WEIGHT_OPTIONS}
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              const num = Number(v);
              Object.assign(d.typography, {fontWeightLight: num});
              propagateWeight(d.typography as unknown as TypographyRecord, 'fontWeightLight', num);
            })
          }
        />
        <SelectRow
          label={t('themes.forms.typography_builder.fields.regular.label', 'Regular')}
          value={String((typo?.fontWeightRegular as number | undefined) ?? 400)}
          options={FONT_WEIGHT_OPTIONS}
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              const num = Number(v);
              Object.assign(d.typography, {fontWeightRegular: num});
              propagateWeight(d.typography as unknown as TypographyRecord, 'fontWeightRegular', num);
            })
          }
        />
        <SelectRow
          label={t('themes.forms.typography_builder.fields.medium.label', 'Medium')}
          value={String((typo?.fontWeightMedium as number | undefined) ?? 500)}
          options={FONT_WEIGHT_OPTIONS}
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              const num = Number(v);
              Object.assign(d.typography, {fontWeightMedium: num});
              propagateWeight(d.typography as unknown as TypographyRecord, 'fontWeightMedium', num);
            })
          }
        />
        <SelectRow
          label={t('themes.forms.typography_builder.fields.bold.label', 'Bold')}
          value={String((typo?.fontWeightBold as number | undefined) ?? 700)}
          options={FONT_WEIGHT_OPTIONS}
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              const num = Number(v);
              Object.assign(d.typography, {fontWeightBold: num});
              propagateWeight(d.typography as unknown as TypographyRecord, 'fontWeightBold', num);
            })
          }
        />
      </ConfigCard>

      <ConfigCard
        title={t('themes.forms.typography_builder.base_sizes.title', 'Base Sizes')}
        defaultOpen={false}
        action={
          <Link
            component="button"
            variant="caption"
            onClick={() => {
              onUpdate((d) => {
                if (!d.typography) return;
                Object.assign(d.typography, DEFAULT_SIZES);
                clearVariantSizes(d.typography as unknown as TypographyRecord);
              });
            }}
            sx={{fontSize: '0.7rem', mr: 1, cursor: 'pointer', background: 'none', border: 'none', p: 0}}
          >
            {t('themes.forms.typography_builder.actions.reset.label', 'Reset')}
          </Link>
        }
      >
        <SliderRow
          label={t('themes.forms.typography_builder.fields.base_font_size.label', 'Base Font Size')}
          value={(typo?.fontSize as number | undefined) ?? 14}
          min={10}
          max={24}
          unit="px"
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              Object.assign(d.typography, {fontSize: v});
              clearVariantSizes(d.typography as unknown as TypographyRecord);
            })
          }
        />
        <SliderRow
          label={t('themes.forms.typography_builder.fields.html_font_size.label', 'HTML Font Size')}
          value={(typo?.htmlFontSize as number | undefined) ?? 16}
          min={10}
          max={24}
          unit="px"
          onChange={(v) =>
            onUpdate((d) => {
              if (!d.typography) return;
              Object.assign(d.typography, {htmlFontSize: v});
              clearVariantSizes(d.typography as unknown as TypographyRecord);
            })
          }
        />
      </ConfigCard>

      <ConfigCard
        title={t('themes.forms.typography_builder.type_scale.title', 'Type Scale')}
        defaultOpen={false}
        action={
          <Link
            component="button"
            variant="caption"
            onClick={() => {
              onUpdate((d) => {
                if (!d.typography) return;
                clearVariantSizes(d.typography as unknown as TypographyRecord);
              });
            }}
            sx={{fontSize: '0.7rem', mr: 1, cursor: 'pointer', background: 'none', border: 'none', p: 0}}
          >
            {t('themes.forms.typography_builder.actions.reset.label', 'Reset')}
          </Link>
        }
      >
        {TYPE_SCALE_VARIANTS.map(({key, label}) => {
          const typoRecord = typo as unknown as Record<string, {fontSize?: string} | undefined>;
          return (
            <Stack key={key} direction="row" alignItems="center" justifyContent="space-between" sx={{py: 0.4}}>
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{fontSize: '0.75rem', fontFamily: 'monospace', minWidth: 72, flexShrink: 0}}
              >
                {label}
              </Typography>
              <TextField
                size="small"
                value={typoRecord?.[key]?.fontSize ?? ''}
                onChange={(e) =>
                  onUpdate((d) => {
                    const typoMap = d.typography as unknown as Record<string, {fontSize?: string} | undefined>;
                    if (typoMap?.[key]) Object.assign(typoMap[key], {fontSize: e.target.value});
                  })
                }
                placeholder={t('themes.forms.typography_builder.fields.type_scale.placeholder', 'e.g. 1.5rem')}
                sx={{width: 110, '& .MuiInputBase-input': {fontSize: '0.75rem', py: 0.5}}}
              />
            </Stack>
          );
        })}
      </ConfigCard>
    </Stack>
  );
}
