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

import {FlowMetadataResponse} from '@thunderid/browser';
import {Context, createContext} from 'react';

export interface FlowMetaContextValue {
  /**
   * Error from the flow meta fetch, if any
   */
  error: Error | null;
  /**
   * Manually re-fetch flow metadata
   */
  fetchFlowMeta: () => Promise<void>;
  /**
   * Whether flow metadata is currently being fetched
   */
  isLoading: boolean;
  /**
   * The fetched flow metadata response, or null while loading / on error
   */
  meta: FlowMetadataResponse | null;
  /**
   * Fetches flow metadata for the given language and activates it in the i18n system.
   * Use this to switch the UI language at runtime.
   */
  switchLanguage: (language: string) => Promise<void>;
}

const FlowMetaContext: Context<FlowMetaContextValue | null> = createContext<FlowMetaContextValue | null>(null);

FlowMetaContext.displayName = 'FlowMetaContext';

export default FlowMetaContext;
