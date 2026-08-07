// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CspOriginHint} from '@thunderid/components';
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
import ColorSelect from '@/features/flows/components/resource-property-panel/ColorSelect';
import PropertySection from '@/features/flows/components/resource-property-panel/PropertySection';
import TextPropertyField from '@/features/flows/components/resource-property-panel/TextPropertyField';
import VariantSelect from '@/features/flows/components/resource-property-panel/VariantSelect';
import ResourcePropertyPanelConstants from '@/features/flows/constants/ResourcePropertyPanelConstants';
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

  const renderElementId = (withClasses = false): ReactElement => (
    <PropertySection key={`${resource.id}-general`} title={sectionTitle('General')}>
      <ResourcePropertyFactory
        key={`${resource.id}-$id`}
        resource={resource}
        propertyKey="id"
        propertyValue={resource.id}
        onChange={handleChange}
      />
      {withClasses && (
        <ClassesPropertyField
          key={`${resource.id}-$classes`}
          resource={resource}
          propertyKey="classes"
          propertyValue={(resource as Element & {classes?: string}).classes ?? ''}
          onChange={handleChange}
        />
      )}
    </PropertySection>
  );

  const sectionTitle = (title: string): string =>
    t(`flows:core.propertiesPanel.sections.${title.toLowerCase()}`, title);

  const renderProperty = ([key, value]: [FieldKey, FieldValue]): ReactElement => (
    <ResourcePropertyFactory
      key={`${resource.id}-${key}`}
      resource={resource}
      propertyKey={key}
      propertyValue={value}
      data-componentid={`${resource.id}-${key}`}
      onChange={handleChange}
    />
  );

  const renderElementPropertyFactory = () => {
    const entries: [FieldKey, FieldValue][] = properties ? Object.entries(properties) : [];
    const grouped: string[] = ResourcePropertyPanelConstants.PROPERTY_SECTIONS.flatMap((section) => section.keys);
    const ungrouped: [FieldKey, FieldValue][] = entries.filter(([key]) => !grouped.includes(key));

    return (
      <>
        {ResourcePropertyPanelConstants.PROPERTY_SECTIONS.map((section) => {
          const sectionEntries: [FieldKey, FieldValue][] = entries.filter(([key]) => section.keys.includes(key));
          // The variant picker is an appearance control, but it renders nothing when
          // the element has no variants. An empty heading would be worse than none.
          const showVariant: boolean = section.title === 'Appearance' && (resource.variants?.length ?? 0) > 0;
          if (sectionEntries.length === 0 && !showVariant) {
            return null;
          }
          return (
            <PropertySection key={section.title} title={sectionTitle(section.title)}>
              {showVariant && (
                <VariantSelect
                  resource={resource}
                  selectedVariant={selectedVariant}
                  onVariantChange={onVariantChange}
                />
              )}
              {sectionEntries.map(renderProperty)}
            </PropertySection>
          );
        })}
        {ungrouped.length > 0 && (
          <PropertySection title={sectionTitle('Other')}>{ungrouped.map(renderProperty)}</PropertySection>
        )}
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
          {renderElementId(true)}
          <FieldExtendedProperties resource={resource} onChange={handleChange} />
          {renderElementPropertyFactory()}
        </>
      );
    case ElementCategories.Action:
      return (
        <>
          {renderElementId(true)}
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
            {renderElementId(true)}
            <TextPropertyField
              resource={resource}
              propertyKey="label"
              propertyValue={(resource as Element & {label?: string}).label ?? ''}
              onChange={(_key, value, res) => handleChange('label', value, res, true)}
            />
            <VariantSelect resource={resource} selectedVariant={selectedVariant} onVariantChange={onVariantChange} />
            <ColorSelect
              resource={resource}
              selectedColor={(resource as Element & {color?: string}).color}
              onColorChange={(newColor: string) => handleChange('color', newColor, resource)}
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
            {renderElementId(true)}
            <TextPropertyField
              resource={resource}
              propertyKey="src"
              propertyValue={(resource as Element & {src?: string}).src ?? ''}
              onChange={(_key, value, res) => handleChange('src', value, res, true)}
            />
            <CspOriginHint value={(resource as Element & {src?: string}).src ?? ''} resourceType="image" />
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
          {renderElementId(true)}
          {renderElementPropertyFactory()}
        </>
      );

    default:
      return (
        <>
          {renderElementId(true)}
          {renderElementPropertyFactory()}
        </>
      );
  }
}

export default memo(ResourceProperties);
