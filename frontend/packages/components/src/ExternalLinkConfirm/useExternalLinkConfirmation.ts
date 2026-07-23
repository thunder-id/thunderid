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

import {useState} from 'react';

/**
 * State and actions for confirming navigation to an external site before it happens.
 *
 * @public
 */
export interface ExternalLinkConfirmationState {
  /** Whether the confirmation dialog should be shown. */
  isOpen: boolean;

  /** The URL awaiting confirmation, or undefined when no navigation is pending. */
  pendingUrl: string | undefined;

  /** Opens the confirmation dialog for the given URL. */
  requestNavigation: (url: string) => void;

  /** Opens the pending URL in a new tab and closes the dialog. */
  confirm: () => void;

  /** Closes the dialog without navigating. */
  cancel: () => void;
}

/**
 * Manages the confirm-before-leaving flow for external links: a link's `onClick` calls
 * `requestNavigation`, and an `ExternalLinkConfirmDialog` driven by the returned state prompts
 * the user before `window.open` is actually called.
 *
 * @public
 */
export default function useExternalLinkConfirmation(): ExternalLinkConfirmationState {
  const [pendingUrl, setPendingUrl] = useState<string | undefined>(undefined);

  return {
    isOpen: pendingUrl !== undefined,
    pendingUrl,
    requestNavigation: (url: string): void => setPendingUrl(url),
    confirm: (): void => {
      if (pendingUrl) {
        window.open(pendingUrl, '_blank', 'noopener,noreferrer');
      }
      setPendingUrl(undefined);
    },
    cancel: (): void => setPendingUrl(undefined),
  };
}
