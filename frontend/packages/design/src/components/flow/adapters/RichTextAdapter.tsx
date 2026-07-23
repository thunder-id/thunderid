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

import {EmbeddedFlowComponentType, EmbeddedFlowEventType, type EmbeddedFlowComponent} from '@thunderid/react';
import {cn, containsMetaTemplate, replaceMetaTemplate} from '@thunderid/utils';
import {Box} from '@wso2/oxygen-ui';
import DOMPurify from 'dompurify';
import type {JSX, MouseEvent as ReactMouseEvent} from 'react';
import useDesign from '../../../contexts/Design/useDesign';
import type {FlowComponent} from '../../../models/flow';

/** The meta key used by the server to embed the application's access URL. */
const APPLICATION_URL_META_KEY = 'application.url';

const REGISTRATION_ENABLED_META_KEY = 'isRegistrationFlowEnabled';

if (typeof window !== 'undefined') {
  DOMPurify.removeHooks('afterSanitizeAttributes');
  DOMPurify.addHook('afterSanitizeAttributes', (node: globalThis.Element) => {
    if (node.tagName === 'A' && node.getAttribute('target') === '_blank') {
      node.setAttribute('rel', 'noopener noreferrer');
    }
  });
}
const RECOVERY_ENABLED_META_KEY = 'isRecoveryFlowEnabled';

/** Checks whether the raw HTML label contains a `data-component-ref` with the given value. */
function hasComponentRef(html: string, ref: string): boolean {
  return html.includes(`data-component-ref="${ref}"`);
}

// When a RICH_TEXT component has a wired action, the anchor's href is decorative — the
// click is intercepted and dispatched as a flow action. Strip navigation attributes so
// hover, middle-click, and right-click "open in new tab" don't leak the placeholder URL.
function neutralizeActionAnchors(html: string, actionRef: string): string {
  const anyHasSentinel = /<a\b[^>]*\sdata-action-ref\s*=/i.test(html);
  return html.replace(/<a\b([^>]*)>/gi, (match: string, attrs: string) => {
    if (anyHasSentinel) {
      const sentinelMatch = /\sdata-action-ref\s*=\s*(?:"([^"]*)"|'([^']*)')/i.exec(attrs);
      const anchorRef = sentinelMatch ? (sentinelMatch[1] ?? sentinelMatch[2]) : undefined;
      if (anchorRef !== actionRef) {
        return match;
      }
    }
    const stripped = attrs
      .replace(/\shref\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
      .replace(/\starget\s*=\s*(?:"[^"]*"|'[^']*')/gi, '')
      .replace(/\srel\s*=\s*(?:"[^"]*"|'[^']*')/gi, '');
    return `<a${stripped} href="#">`;
  });
}

interface RichTextAdapterProps {
  component: FlowComponent;
  resolve: (template: string | undefined) => string | undefined;
  /**
   * Current form values, passed to `onSubmit` when a wired anchor click
   * dispatches the component's `action`.
   */
  values?: Record<string, string>;
  /**
   * Fired when the rich text carries a wired `action` and one of its anchors is
   * clicked. The adapter synthesizes an ACTION-shaped component whose `id`
   * matches the action ref so the caller can dispatch `flow/execute` as it would
   * for a real button.
   */
  onSubmit?: (action: EmbeddedFlowComponent, inputs: Record<string, string>) => void;
}

export default function RichTextAdapter({
  component,
  resolve,
  values = undefined,
  onSubmit = undefined,
}: RichTextAdapterProps): JSX.Element | null {
  const {isDesignEnabled} = useDesign();
  const rawLabel = typeof component.label === 'string' ? component.label : undefined;
  const richTextAction = component.action;

  // When the component carries a wired `action`, intercept anchor clicks inside the
  // sanitized HTML and dispatch as a flow action instead of following the anchor's
  // native href. This takes precedence over the URL-sniffing branches below, which
  // are the pre-action-wiring workaround for authoring simple sign-up/recovery
  // links from a template.
  if (richTextAction?.ref && rawLabel) {
    const resolvedLabel = resolve(rawLabel) ?? rawLabel;

    if (hasComponentRef(resolvedLabel, 'self-sign-up-link')) {
      if (resolve(`{{meta(${REGISTRATION_ENABLED_META_KEY})}}`) !== 'true') {
        return null;
      }
    }
    if (hasComponentRef(resolvedLabel, 'recovery-link')) {
      if (resolve(`{{meta(${RECOVERY_ENABLED_META_KEY})}}`) !== 'true') {
        return null;
      }
    }

    const actionRef = richTextAction.ref;

    const handleClick = (event: ReactMouseEvent<HTMLDivElement>): void => {
      const target = event.target;
      if (!(target instanceof Element)) {
        return;
      }
      const anchor = target.closest('a');
      if (!anchor) {
        return;
      }
      const containerHasSentinel = event.currentTarget.querySelector('a[data-action-ref]') !== null;
      if (containerHasSentinel) {
        const anchorRef = anchor.getAttribute('data-action-ref');
        if (!anchorRef || anchorRef !== actionRef) {
          return;
        }
      }
      event.preventDefault();
      if (!onSubmit) {
        return;
      }
      const syntheticAction: EmbeddedFlowComponent = {
        eventType: EmbeddedFlowEventType.Trigger,
        id: actionRef,
        ref: actionRef,
        type: EmbeddedFlowComponentType.Action,
      };
      onSubmit(syntheticAction, values ?? {});
    };

    const sanitized = DOMPurify.sanitize(resolvedLabel, {
      ADD_ATTR: ['target', 'data-action-ref', 'data-component-ref'],
    });
    const finalHtml = neutralizeActionAnchors(sanitized, actionRef);

    return (
      <Box
        id={component.id}
        className={[cn('Flow--richText'), component.classes].filter(Boolean).join(' ')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        onClick={handleClick}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: finalHtml}}
      />
    );
  }

  // When any component label embeds the application's URL, render it only when
  // the URL is present. There is no sensible local fallback route for this.
  if (rawLabel && containsMetaTemplate(rawLabel, APPLICATION_URL_META_KEY)) {
    const resolvedUrl = resolve(`{{meta(${APPLICATION_URL_META_KEY})}}`);

    if (!resolvedUrl || containsMetaTemplate(resolvedUrl, APPLICATION_URL_META_KEY)) {
      return null;
    }

    let resolvedLabel = resolve(rawLabel) ?? rawLabel;

    if (containsMetaTemplate(resolvedLabel, APPLICATION_URL_META_KEY)) {
      resolvedLabel = replaceMetaTemplate(resolvedLabel, APPLICATION_URL_META_KEY, resolvedUrl);
    }

    return (
      <Box
        id={component.id}
        className={[cn('Flow--richText'), component.classes].filter(Boolean).join(' ')}
        sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
        // eslint-disable-next-line react/no-danger
        dangerouslySetInnerHTML={{__html: DOMPurify.sanitize(resolvedLabel)}}
      />
    );
  }

  const resolvedLabel = resolve(rawLabel);

  return (
    <Box
      id={component.id}
      className={[cn('Flow--richText'), component.classes].filter(Boolean).join(' ')}
      sx={{mb: 1, textAlign: isDesignEnabled ? 'center' : 'left'}}
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={{
        __html: DOMPurify.sanitize(resolvedLabel ?? rawLabel ?? '', {ADD_ATTR: ['target']}),
      }}
    />
  );
}
