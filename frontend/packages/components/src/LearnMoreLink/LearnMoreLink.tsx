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

import {useConfig} from '@thunderid/contexts';
import {Link} from '@wso2/oxygen-ui';
import {ExternalLink as ExternalLinkIcon} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ExternalLinkConfirmDialog from '../ExternalLinkConfirm/ExternalLinkConfirmDialog';
import useExternalLinkConfirmation from '../ExternalLinkConfirm/useExternalLinkConfirmation';

/**
 * Props for {@link LearnMoreLink}.
 *
 * @public
 */
export interface LearnMoreLinkProps {
  /** Section id to look up in `documentation.links`, e.g. "users", "applications". */
  docKey: string;

  /** Optional label override. Defaults to the `common:learnMore` translation. */
  label?: string;
}

/**
 * Renders a "Learn more" link to the documentation page configured for `docKey`. Renders
 * nothing when no link is configured for that key, so pages can add this unconditionally and
 * the link only appears once an operator configures a URL for it.
 *
 * Clicking the link prompts the user with an {@link ExternalLinkConfirmDialog} before
 * navigating away.
 *
 * @public
 */
export default function LearnMoreLink({docKey, label = undefined}: LearnMoreLinkProps): JSX.Element | null {
  const {t} = useTranslation();
  const {getDocumentationLink} = useConfig();
  const {isOpen, pendingUrl, requestNavigation, confirm, cancel} = useExternalLinkConfirmation();

  const href = getDocumentationLink(docKey);

  if (!href) {
    return null;
  }

  return (
    <>
      <Link
        component="button"
        type="button"
        onClick={() => requestNavigation(href)}
        style={{
          color: 'inherit',
          fontWeight: 600,
          display: 'inline-flex',
          alignItems: 'center',
          gap: 2,
          background: 'none',
          border: 'none',
          padding: 0,
          cursor: 'pointer',
        }}
      >
        {label ?? t('common:learnMore')}
        <ExternalLinkIcon size={12} style={{flexShrink: 0, opacity: 0.7}} />
      </Link>
      <ExternalLinkConfirmDialog isOpen={isOpen} pendingUrl={pendingUrl} onConfirm={confirm} onCancel={cancel} />
    </>
  );
}
