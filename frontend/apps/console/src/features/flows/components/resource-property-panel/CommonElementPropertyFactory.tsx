/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useLogger} from '@thunderid/logger/react';
import {
  Autocomplete,
  type AutocompleteRenderInputParams,
  Box,
  FormControl,
  FormLabel,
  MenuItem,
  Select,
  TextField,
} from '@wso2/oxygen-ui';
import startCase from 'lodash-es/startCase';
import {type ComponentType, type ReactElement, useState, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxPropertyField from './CheckboxPropertyField';
import RichTextActionFields from './rich-text/RichTextActionFields';
import RichTextWithTranslation from './rich-text/RichTextWithTranslation';
import TextPropertyField from './TextPropertyField';
import FlowBuilderElementConstants from '../../constants/FlowBuilderElementConstants';
import {ElementTypes} from '../../models/elements';
import type {Resource} from '../../models/resources';

const TEXT_ALIGN_VALUES = ['left', 'center', 'right', 'justify', 'inherit'] as const;

// ---------------------------------------------------------------------------
// Lazy icon loading — loaded once on first picker open, then cached.
// Using a type-only import avoids pulling the entire icons bundle into every
// module that imports CommonElementPropertyFactory.
// ---------------------------------------------------------------------------

type IconModule = typeof import('@wso2/oxygen-ui-icons-react');

let cachedModule: IconModule | null = null;
let cachedNames: string[] | null = null;

function buildIconNames(mod: IconModule): string[] {
  cachedNames ??= Object.keys(mod).filter((k) => {
    if (!/^[A-Z]/.test(k)) return false;
    const v = mod[k as keyof IconModule] as unknown;
    // Lucide icons are forwardRef exotic objects with a displayName.
    // The base Icon export is excluded (no displayName).
    return (
      typeof v === 'object' &&
      v !== null &&
      '$$typeof' in v &&
      typeof (v as Record<string, unknown>).displayName === 'string'
    );
  });
  return cachedNames;
}

async function loadIconModule(): Promise<{module: IconModule; names: string[]}> {
  cachedModule ??= await import('@wso2/oxygen-ui-icons-react');
  return {module: cachedModule, names: buildIconNames(cachedModule)};
}

// ---------------------------------------------------------------------------
// IconPickerField — extracted so hooks are always called at the top level.
// ---------------------------------------------------------------------------

interface IconPickerFieldProps {
  resource: Resource;
  propertyKey: string;
  propertyValue: unknown;
  onChange: (propertyKey: string, newValue: unknown, resource: Resource, debounce?: boolean) => void;
}

function IconPickerField({resource, propertyKey, propertyValue, onChange}: IconPickerFieldProps): ReactElement {
  const logger = useLogger('ConfigureDetails');
  const [loaded, setLoaded] = useState<{module: IconModule; names: string[]} | null>(null);

  const handleOpen = useCallback(() => {
    if (!loaded) {
      loadIconModule()
        .then(setLoaded)
        .catch((error) => {
          logger.error('Failed to load icon module', {error});
        });
    }
  }, [loaded, logger]);

  const iconNames = loaded?.names ?? [];
  const totalCount = iconNames.length;

  // Only resolve the preview icon once the module is loaded.
  const IconPreview: ComponentType<{size?: number}> | undefined =
    loaded && typeof propertyValue === 'string' && propertyValue
      ? (loaded.module[propertyValue as keyof IconModule] as ComponentType<{size?: number}> | undefined)
      : undefined;

  return (
    <Box>
      <Autocomplete
        options={iconNames}
        value={typeof propertyValue === 'string' ? propertyValue : null}
        loading={!loaded}
        onOpen={handleOpen}
        onChange={(_event: React.SyntheticEvent, newValue: string | null) => {
          onChange(propertyKey, newValue ?? '', resource);
        }}
        // Cap rendered DOM nodes: show first 100 when browsing, first 100
        // matches when searching. This avoids rendering 1000+ list items.
        filterOptions={(options, {inputValue}) => {
          const query = inputValue.trim().toLowerCase();
          const filtered = query ? options.filter((n) => n.toLowerCase().includes(query)) : options;
          return filtered.slice(0, 100);
        }}
        noOptionsText={loaded ? 'No icons found' : 'Loading icons…'}
        renderInput={(params: AutocompleteRenderInputParams) => {
          const {InputProps: acInputProps, inputProps: acHtmlInputProps, InputLabelProps, ...restParams} = params;
          return (
            <TextField
              {...restParams}
              label={startCase(propertyKey)}
              size="small"
              placeholder={loaded ? `Search ${totalCount} icons…` : 'Loading icons…'}
              slotProps={{
                input: {
                  ...acInputProps,
                  startAdornment: IconPreview ? (
                    <Box sx={{display: 'flex', alignItems: 'center', pl: 0.5, pr: 0.5}}>
                      <IconPreview size={16} />
                    </Box>
                  ) : (
                    acInputProps?.startAdornment
                  ),
                },
                htmlInput: acHtmlInputProps,
                inputLabel: InputLabelProps,
              }}
            />
          );
        }}
        renderOption={({key, ...props}: React.HTMLAttributes<HTMLLIElement> & {key: string}, option: string) => {
          const Icon =
            loaded && (loaded.module[option as keyof IconModule] as ComponentType<{size?: number}> | undefined);
          return (
            <li key={key} {...props}>
              <Box display="flex" alignItems="center" gap={1}>
                {Icon && <Icon size={16} />}
                {option}
              </Box>
            </li>
          );
        }}
      />
    </Box>
  );
}

// ---------------------------------------------------------------------------
// CommonElementPropertyFactory — main factory
// ---------------------------------------------------------------------------

/**
 * Props interface of {@link CommonElementPropertyFactory}
 */
export interface CommonElementPropertyFactoryPropsInterface {
  resource: Resource;
  propertyKey: string;
  propertyValue: unknown;
  onChange: (propertyKey: string, newValue: unknown, resource: Resource, debounce?: boolean) => void;
  [key: string]: unknown;
}

/**
 * Factory to generate the common property configurator for the given element.
 *
 * @param props - Props injected to the component.
 * @returns The CommonElementPropertyFactory component.
 */
function CommonElementPropertyFactory({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: CommonElementPropertyFactoryPropsInterface): ReactElement | null {
  const {t} = useTranslation();
  if (propertyKey === 'label') {
    if (resource.type === ElementTypes.RichText) {
      return (
        <>
          <RichTextWithTranslation
            onChange={(html: string) => onChange(propertyKey, html, resource, true)}
            resource={resource}
            {...rest}
          />
          <RichTextActionFields resource={resource} onChange={onChange} />
        </>
      );
    }
  }

  if (resource.type === ElementTypes.Icon && propertyKey === 'name') {
    return (
      <IconPickerField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
      />
    );
  }

  if (resource.type === ElementTypes.Text && propertyKey === 'align') {
    return (
      <FormControl fullWidth size="small">
        <FormLabel htmlFor={propertyKey}>{t('flows:core.elements.text.align.label')}</FormLabel>
        <Select
          id={propertyKey}
          value={typeof propertyValue === 'string' ? propertyValue : 'left'}
          onChange={(e) => onChange(propertyKey, e.target.value, resource)}
        >
          {TEXT_ALIGN_VALUES.map((value) => (
            <MenuItem key={value} value={value}>
              {t(`flows:core.elements.text.align.options.${value}`)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    );
  }

  if (typeof propertyValue === 'boolean') {
    return (
      <CheckboxPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'number') {
    return (
      <TextPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={String(propertyValue)}
        onChange={(key, value, res) => onChange(key, value !== '' ? Number(value) : 0, res, true)}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'string') {
    return (
      <TextPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
        {...rest}
      />
    );
  }

  if (resource.type === ElementTypes.Captcha) {
    return (
      <TextField
        fullWidth
        label="Provider"
        defaultValue={FlowBuilderElementConstants.DEFAULT_CAPTCHA_PROVIDER}
        slotProps={{
          htmlInput: {
            disabled: true,
            readOnly: true,
          },
        }}
        {...rest}
      />
    );
  }

  return null;
}

export default CommonElementPropertyFactory;
