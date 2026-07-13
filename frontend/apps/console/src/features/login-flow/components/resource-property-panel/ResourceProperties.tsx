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

import {FormControl, FormLabel, MenuItem, Select} from '@wso2/oxygen-ui';
import {memo, useCallback, useMemo, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ButtonExtendedProperties from './extended-properties/ButtonExtendedProperties';
import CallProperties from './extended-properties/CallProperties';
import ExecutionExtendedProperties from './extended-properties/ExecutionExtendedProperties';
import FieldExtendedProperties from './extended-properties/FieldExtendedProperties';
import RulesProperties from './nodes/RulesProperties';
import ResourcePropertyFactory from './ResourcePropertyFactory';
import ClassesPropertyField from '@/features/flows/components/resource-property-panel/ClassesPropertyField';
import TextPropertyField from '@/features/flows/components/resource-property-panel/TextPropertyField';
import VariantSelect from '@/features/flows/components/resource-property-panel/VariantSelect';
import type {ResourcePropertiesProps} from '@/features/flows/context/FlowBuilderCoreProvider';
import type {FieldKey, FieldValue} from '@/features/flows/models/base';
import {ElementCategories, ElementTypes, type Element} from '@/features/flows/models/elements';
import type {Resource} from '@/features/flows/models/resources';
import {StepCategories, StepTypes} from '@/features/flows/models/steps';

/**
 * Factory to generate the property configurator for the given password recovery flow resource.
 *
 * @param props - Props injected to the component.
 * @returns The ResourceProperties component.
 */
const coerceValue = (newValue: unknown): string | boolean | number | object => {
  if (typeof newValue === 'boolean') {
    return newValue;
  }
  if (typeof newValue === 'number') {
    return newValue;
  }
  if (typeof newValue === 'object' && newValue !== null) {
    return newValue;
  }
  if (typeof newValue === 'string') {
    return String(newValue);
  }
  return '';
};

function ResourceProperties({
  properties,
  resource,
  onChange,
  onVariantChange,
}: ResourcePropertiesProps): ReactElement | null {
  const {t} = useTranslation();

  const handleChange = useCallback(
    (propertyKey: string, newValue: unknown, changedResource: unknown, debounce?: boolean): void => {
      onChange(propertyKey, coerceValue(newValue), changedResource as Resource, debounce);
    },
    [onChange],
  );
  const selectedVariant = useMemo<Element | undefined>(() => {
    if (!resource?.variants || resource.variants.length === 0) {
      return undefined;
    }
    return resource.variants.find((v: Element) => v.variant === (resource as Element).variant) as Element | undefined;
  }, [resource]);

  const renderElementId = (): ReactElement => (
    <ResourcePropertyFactory
      key={`${resource.id}-$id`}
      resource={resource}
      propertyKey="id"
      propertyValue={resource.id}
      onChange={handleChange}
    />
  );

  const renderElementClasses = (): ReactElement => (
    <ClassesPropertyField
      key={`${resource.id}-$classes`}
      resource={resource}
      propertyKey="classes"
      propertyValue={(resource as Element & {classes?: string}).classes ?? ''}
      onChange={handleChange}
    />
  );

  const renderElementPropertyFactory = () => {
    return (
      <>
        <VariantSelect resource={resource} selectedVariant={selectedVariant} onVariantChange={onVariantChange} />
        {properties &&
          Object.entries(properties)?.map(([key, value]: [FieldKey, FieldValue]) => (
            <ResourcePropertyFactory
              key={`${resource.id}-${key}`}
              resource={resource}
              propertyKey={key}
              propertyValue={value}
              data-componentid={`${resource.id}-${key}`}
              onChange={handleChange}
            />
          ))}
      </>
    );
  };

  switch (resource.category) {
    case StepCategories.Interface:
      if (resource.type === StepTypes.End) {
        return (
          <>
            {renderElementId()}
            {/* <FlowCompletionProperties resource={resource} onChange={onChange} /> */}
          </>
        );
      }

      return null;
    case ElementCategories.Field:
      return (
        <>
          {renderElementId()}
          {renderElementClasses()}
          <FieldExtendedProperties resource={resource} onChange={handleChange} />
          {renderElementPropertyFactory()}
        </>
      );
    case ElementCategories.Action:
      return (
        <>
          {renderElementId()}
          {renderElementClasses()}
          {resource.type === ElementTypes.Action && (
            <ButtonExtendedProperties resource={resource} onChange={handleChange} onVariantChange={onVariantChange} />
          )}
          {renderElementPropertyFactory()}
        </>
      );
    case StepCategories.Decision:
      if (resource.type === StepTypes.Rule) {
        return (
          <>
            {renderElementId()}
            <RulesProperties />
          </>
        );
      }

      return null;
    case StepCategories.Workflow:
      if (resource.type === StepTypes.Call) {
        return (
          <>
            {renderElementId()}
            <CallProperties resource={resource} onChange={handleChange} />
          </>
        );
      }
      return (
        <>
          {renderElementId()}
          <ExecutionExtendedProperties resource={resource} onChange={handleChange} />
        </>
      );
    case ElementCategories.Display:
      if (resource.type === ElementTypes.Text) {
        return (
          <>
            {renderElementId()}
            {renderElementClasses()}
            <VariantSelect resource={resource} selectedVariant={selectedVariant} onVariantChange={onVariantChange} />
            <TextPropertyField
              resource={resource}
              propertyKey="label"
              propertyValue={(resource as Element & {label?: string}).label ?? ''}
              onChange={(_key, value, res) => handleChange('label', value, res, true)}
            />
            <FormControl fullWidth size="small">
              <FormLabel htmlFor="align-select">{t('flows:core.elements.text.align.label')}</FormLabel>
              <Select
                id="align-select"
                value={(resource as Element & {align?: string}).align ?? 'left'}
                onChange={(e) => handleChange('align', e.target.value, resource)}
              >
                {(['left', 'center', 'right', 'justify', 'inherit'] as const).map((opt) => (
                  <MenuItem key={opt} value={opt}>
                    {t(`flows:core.elements.text.align.options.${opt}`)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </>
        );
      }
      if (resource.type === ElementTypes.Image) {
        return (
          <>
            {renderElementId()}
            {renderElementClasses()}
            <TextPropertyField
              resource={resource}
              propertyKey="src"
              propertyValue={(resource as Element & {src?: string}).src ?? ''}
              onChange={(_key, value, res) => handleChange('src', value, res, true)}
            />
            <TextPropertyField
              resource={resource}
              propertyKey="alt"
              propertyValue={(resource as Element & {alt?: string}).alt ?? ''}
              onChange={(_key, value, res) => handleChange('alt', value, res, true)}
            />
            <TextPropertyField
              resource={resource}
              propertyKey="width"
              propertyValue={(resource as Element & {width?: string}).width ?? ''}
              onChange={(_key, value, res) => handleChange('width', value, res, true)}
            />
            <TextPropertyField
              resource={resource}
              propertyKey="height"
              propertyValue={(resource as Element & {height?: string}).height ?? ''}
              onChange={(_key, value, res) => handleChange('height', value, res, true)}
            />
          </>
        );
      }
      return (
        <>
          {renderElementId()}
          {renderElementClasses()}
          {renderElementPropertyFactory()}
        </>
      );

    default:
      return (
        <>
          {renderElementId()}
          {renderElementClasses()}
          {renderElementPropertyFactory()}
        </>
      );
  }
}

export default memo(ResourceProperties);
