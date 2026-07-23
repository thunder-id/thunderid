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

import {deriveEventType, shouldPromoteToSubmit} from './reactFlowTransformer';
import {containsTemplateLiteral} from '../components/resources/elements/adapters/TemplatePlaceholder';
import {ActionEventTypes, ElementCategories, ElementTypes, type Element} from '../models/elements';
import type {Application} from '@/features/applications/models/application';

/**
 * A flow element as consumed by the simulation preview, with the presentation
 * properties the renderers read.
 */
export interface PreviewComponent extends Element {
  label?: string;
  placeholder?: string;
  src?: string;
  alt?: string;
  align?: 'inherit' | 'left' | 'center' | 'right' | 'justify';
  image?: string;
}

const APPLICATION_META_PATTERN = /\{\{\s*meta\(application\.(\w+)\)\s*\}\}/g;

/**
 * Deeply resolves `{{ meta(application.*) }}` placeholders in a component tree
 * against the selected application (e.g. logoUrl, name).
 */
export function resolveApplicationMeta<T>(value: T, application: Application): T {
  if (typeof value === 'string') {
    return value.replaceAll(APPLICATION_META_PATTERN, (_match, property: string) => {
      const resolved = (application as unknown as Record<string, unknown>)[property];
      return typeof resolved === 'string' ? resolved : '';
    }) as unknown as T;
  }
  if (Array.isArray(value)) {
    return (value as unknown[]).map((item: unknown) => resolveApplicationMeta(item, application)) as unknown as T;
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, entry]) => [
        key,
        resolveApplicationMeta(entry, application),
      ]),
    ) as unknown as T;
  }
  return value;
}

/**
 * Deeply resolves i18n templates (`{{ t(...) }}`) in a component tree using the
 * provided text resolver, so the gate renderer receives display-ready labels
 * instead of raw translation keys it cannot resolve inside the console.
 */
export function resolveTemplatesDeep(value: unknown, resolveText: (raw: string) => string): unknown {
  if (typeof value === 'string') {
    return containsTemplateLiteral(value) ? resolveText(value) : value;
  }
  if (Array.isArray(value)) {
    return (value as unknown[]).map((item: unknown) => resolveTemplatesDeep(item, resolveText));
  }
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>).map(([key, entry]) => [
        key,
        resolveTemplatesDeep(entry, resolveText),
      ]),
    );
  }
  return value;
}

function escapeHtml(raw: string): string {
  return raw.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;').replaceAll('"', '&quot;');
}

/**
 * Skeleton markup for runtime-resolved input fields, mirroring the consent
 * placeholder's look: a dashed box sketching mock fields (label bar + input
 * outline) with an italic caption. Rendered through the gate's RICH_TEXT
 * adapter, so `currentColor`-based tints adapt to the applied theme.
 */
function dynamicFieldsSkeletonHtml(caption: string): string {
  // color-mix over currentColor keeps the borders tracking the applied theme's
  // text color, like the label bars — MUI palette tokens don't reach this HTML.
  const line = 'color-mix(in srgb, currentColor 40%, transparent)';
  const field = (labelWidth: string): string =>
    '<div style="margin-bottom:12px;">' +
    `<div style="width:${labelWidth};height:6px;border-radius:4px;background:currentColor;opacity:0.25;margin-bottom:7px;"></div>` +
    `<div style="height:36px;border:1px solid ${line};border-radius:6px;"></div>` +
    '</div>';
  return (
    `<div style="border:1px dashed ${line};border-radius:8px;padding:12px 14px 0;">` +
    `${field('35%')}${field('50%')}` +
    '</div>' +
    `<div style="text-align:center;font-style:italic;opacity:0.65;font-size:0.75em;margin:7px 0 0;">${escapeHtml(caption)}</div>`
  );
}

/**
 * Derives the runtime `eventType` for action components that don't carry one,
 * mirroring the save-time transformation (see `cleanComponents` in
 * reactFlowTransformer). The gate's block renderer only renders actions with an
 * `eventType`, so designer components without one would lose their buttons in
 * the themed preview.
 */
export function withDerivedEventTypes(list: PreviewComponent[], promoteSubmit = false): PreviewComponent[] {
  return list.map((component: PreviewComponent) => {
    let next = component;
    if (component.category === ElementCategories.Action && !(component as {eventType?: string}).eventType) {
      let eventType = deriveEventType(component as Element & {buttonType?: string});
      if (promoteSubmit && eventType === ActionEventTypes.Trigger) {
        eventType = ActionEventTypes.Submit;
      }
      next = {...component, eventType} as PreviewComponent;
    }
    if (next.components?.length) {
      const children = next.components as PreviewComponent[];
      next = {...next, components: withDerivedEventTypes(children, shouldPromoteToSubmit(children))};
    }
    return next;
  });
}

/**
 * Replaces dynamic input placeholders with a skeleton sketch of the fields.
 * The server resolves these into actual input fields at runtime, so the
 * preview shows a placeholder where the runtime ones will appear. The canvas
 * element's own placeholder/hint texts ("Dynamic Input", …) are builder
 * chrome and are blanked out.
 */
export function withDynamicFieldStandIns(list: PreviewComponent[], caption: string): PreviewComponent[] {
  return list.map((component: PreviewComponent) => {
    if (component.type === ElementTypes.DynamicInputPlaceholder) {
      return {
        ...component,
        type: ElementTypes.RichText,
        label: dynamicFieldsSkeletonHtml(caption),
        placeholder: '',
        hint: '',
      } as PreviewComponent;
    }
    if (component.components?.length) {
      return {...component, components: withDynamicFieldStandIns(component.components as PreviewComponent[], caption)};
    }
    return component;
  });
}

/**
 * Re-attaches the SDK-facing `action.ref` to RICH_TEXT components from the
 * simulation options. A rich-text link's option carries the component id as
 * `sourceComponentId` and the wiring ref as `edgeId`; the gate's RichTextAdapter
 * only dispatches an anchor click when the component exposes `action.ref`, and
 * that field can be lost by the time a loaded flow reaches the preview. Sourcing
 * it from the simulation graph makes link clicks work regardless.
 */
export function withRichTextActionRefs(
  list: PreviewComponent[],
  optionsByComponentId: Map<string, string>,
): PreviewComponent[] {
  return list.map((component: PreviewComponent) => {
    let next = component;
    const ref = component.type === ElementTypes.RichText ? optionsByComponentId.get(component.id) : undefined;
    if (ref && (component as {action?: {ref?: string}}).action?.ref !== ref) {
      next = {...component, action: {...(component as {action?: object}).action, ref}} as PreviewComponent;
    }
    if (next.components?.length) {
      next = {
        ...next,
        components: withRichTextActionRefs(next.components as PreviewComponent[], optionsByComponentId),
      };
    }
    return next;
  });
}
