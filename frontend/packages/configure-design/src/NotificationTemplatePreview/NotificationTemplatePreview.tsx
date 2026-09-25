// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DefaultTheme, type Theme} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {Box} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import GatePreview from '../GatePreview/GatePreview';

/** Delivery channel a notification template targets. */
export type NotificationChannel = 'email' | 'sms';

/**
 * Resolves `{{ meta(application.*) }}` placeholders against the selected application
 * (e.g. name, logoUrl). Mirrors the flow builder's application-meta resolution so a
 * template previews with the same branding values a real notification would use.
 */
const APPLICATION_META_PATTERN = /\{\{\s*meta\(application\.(\w+)\)\s*\}\}/g;

export interface NotificationTemplatePreviewProps {
  /** The delivery channel — controls the frame (browser for email, phone for SMS) and rendering. */
  channel: NotificationChannel;
  /** Raw subject line (email only), with placeholders intact. */
  subject?: string;
  /** Raw body — HTML for email, plain text for SMS — with placeholders intact. */
  body: string;
  /**
   * Resolved application design theme (from `useGetDesignResolve`). The caller fetches
   * it so this component stays presentational and free of data-fetch dependencies.
   * Null renders a loading spinner; falls back to the default theme when unset.
   */
  theme?: Theme | null;
  /**
   * The selected application, used to resolve `{{ meta(application.*) }}` placeholders
   * (e.g. `name`, `logoUrl`). Caller-provided to avoid a dependency on the applications
   * package from this design package.
   */
  application?: Record<string, unknown>;
  /** Color scheme to render the preview in. */
  colorScheme?: 'light' | 'dark';
}

/**
 * Renders a live preview of a notification template (email or SMS) using the same
 * resolution + theming pipeline the flow builder uses for gate screens:
 *
 * - `{{ t(...) }}` translations resolve via i18n (react-i18next),
 * - `{{ meta(application.*) }}` resolves against the selected application,
 * - branding/design tokens apply from the resolved application theme (via GatePreview's
 *   themed iframe).
 *
 * `{{ ctx(...) }}` runtime placeholders are intentionally left intact — they are filled
 * with real data only when the notification is actually sent.
 *
 * The component is presentational: it takes the raw template content plus the already
 * resolved theme/application, so both the flow builder's Send Email/SMS step preview and
 * the notification template management editor can reuse it, each supplying content from
 * its own source (the template API vs. the editor's local state) and updating in real time.
 */
export default function NotificationTemplatePreview({
  channel,
  subject = undefined,
  body,
  theme = undefined,
  application = undefined,
  colorScheme = undefined,
}: NotificationTemplatePreviewProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveAll} = useTemplateLiteralResolver();

  const resolve = useMemo(() => {
    return (raw?: string): string => {
      if (!raw) {
        return '';
      }
      // {{ t(...) }} via i18n, then {{ meta(application.*) }} via the selected application.
      let out = resolveAll(raw, {t}) ?? raw;
      if (application) {
        out = out.replaceAll(APPLICATION_META_PATTERN, (_match, property: string) => {
          const value = application[property];
          return typeof value === 'string' ? value : '';
        });
      }
      return out;
    };
  }, [resolveAll, t, application]);

  const mock: EmbeddedFlowComponent[] = useMemo(() => {
    const components: EmbeddedFlowComponent[] = [];
    if (channel === 'email' && subject) {
      components.push({
        id: 'notification-subject',
        type: 'TEXT',
        label: resolve(subject),
      } as unknown as EmbeddedFlowComponent);
    }
    components.push({
      id: 'notification-body',
      // Email bodies are HTML (RICH_TEXT); SMS bodies are plain text (TEXT).
      type: channel === 'email' ? 'RICH_TEXT' : 'TEXT',
      label: resolve(body),
    } as unknown as EmbeddedFlowComponent);
    return components;
  }, [channel, subject, body, resolve]);

  // Render as a floating window (centered, elevated card) rather than the gate's docked
  // browser/phone "tab" chrome. GatePreview is frameless so it contributes only the
  // themed content; the floating frame is this component's own container.
  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'flex-start',
        width: '100%',
        p: 2,
        boxSizing: 'border-box',
      }}
    >
      <Box
        sx={{
          width: '100%',
          maxWidth: channel === 'sms' ? 360 : 640,
          minHeight: channel === 'sms' ? 560 : 420,
          borderRadius: 2,
          boxShadow: 8,
          overflow: 'hidden',
          bgcolor: 'background.paper',
        }}
      >
        <GatePreview
          frameless
          theme={theme === undefined ? (DefaultTheme as Theme) : theme}
          baseTheme={DefaultTheme as Theme}
          mock={mock}
          colorScheme={colorScheme}
          showToolbar={false}
        />
      </Box>
    </Box>
  );
}
