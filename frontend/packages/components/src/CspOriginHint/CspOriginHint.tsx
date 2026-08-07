// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {Alert} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {resolveCspHint, type CspResourceType} from './resolveCspHint';
import ExternalLink from '../ExternalLink/ExternalLink';

// useConfig throws without a ConfigProvider, so resolve defensively: the docs link is added only
// when documentation config is available; the guidance text always renders.
function useHasCspDocLink(): boolean {
  try {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    return Boolean(useConfig().getDocumentationLink('deployment.csp'));
  } catch {
    return false;
  }
}

// English default for each message, passed as the t() default value. Canonical copy is in the i18n
// locale files; these are the fallback when no i18next instance is configured (for example in tests).
const CSP_HINT_DEFAULTS: Record<string, string> = {
  'common:csp.hint.image':
    "This image loads from {{origin}}. Add {{origin}} to the img-src directive of your server's Content Security Policy so it is not blocked.",
  'common:csp.hint.stylesheet':
    "This stylesheet loads from {{origin}}. Add {{origin}} to the style-src-elem directive of your server's Content Security Policy so it is not blocked.",
  'common:csp.hint.font':
    "This font loads from {{origin}}. Add {{origin}} to the style-src-elem directive, and the origin that serves its font files to the font-src directive, of your server's Content Security Policy so it is not blocked.",
  'common:csp.hint.fontGoogle':
    "Google Fonts loads its stylesheet from fonts.googleapis.com and its font files from fonts.gstatic.com. Add fonts.googleapis.com to the style-src-elem directive and fonts.gstatic.com to the font-src directive of your server's Content Security Policy so this font is not blocked.",
};

/**
 * Props for {@link CspOriginHint}.
 *
 * @public
 */
export interface CspOriginHintProps {
  /** The current value of the field, typically a URL. */
  value: string;

  /** The resource type the value is loaded as, used to name the directive to allow it in. */
  resourceType: CspResourceType;
}

/**
 * Inline advisory shown when a field's value loads from an external origin, naming the CSP directive
 * (`img-src`, `style-src-elem`, or `font-src`) to allow it in and linking to the docs. Renders
 * nothing for same-origin URLs, relative paths, `data:` URIs, or font-family names, so callers can
 * render it unconditionally next to a field.
 *
 * @public
 */
export default function CspOriginHint({value, resourceType}: CspOriginHintProps): JSX.Element | null {
  const {t} = useTranslation();
  const hasDocLink = useHasCspDocLink();
  const hint = resolveCspHint(value, resourceType);

  if (!hint) {
    return null;
  }

  return (
    <Alert severity="info" sx={{mt: 1}} data-componentid="csp-origin-hint">
      {t(hint.messageKey, CSP_HINT_DEFAULTS[hint.messageKey], {origin: hint.origin})}{' '}
      {hasDocLink ? <ExternalLink docKey="deployment.csp" confirmBeforeNavigate={false} /> : null}
    </Alert>
  );
}
