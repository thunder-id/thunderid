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

import type {JSX, ReactNode} from 'react';

/**
 * Validation step status during configuration import
 */
export interface ValidationStep {
  id: string;
  label: string;
  status: 'pending' | 'validating' | 'completed' | 'failed';
}

/**
 * Parse error details for failed configuration sections
 */
export interface ParseError {
  resourceType: string;
  fileName: string;
  error: string;
}

/**
 * Configuration summary item for display
 */
export interface ConfigSummaryItem {
  id: string;
  icon: JSX.Element;
  label: string;
  value: string | number;
  content?: ReactNode;
  /**
   * Status for export items (optional, used in export flow)
   */
  status?: 'ready' | 'warning';
  /**
   * Dependency count for export items (optional, used in export flow)
   */
  dependencyCount?: number;
}

/**
 * Product configuration structure
 */
export interface ProductConfig {
  application?: unknown[];
  connection?: unknown[];
  user_type?: unknown[];
  organization_unit?: unknown[];
  user?: unknown[];
  flow?: unknown[];
  translation?: unknown[];
  layout?: unknown[];
  theme?: unknown[];
  [key: string]: unknown;
}

/**
 * Import request payload for /import endpoint.
 */
export interface ImportRequest {
  content: string;
  variables?: Record<string, string | string[]>;
  dryRun?: boolean;
  options?: {
    upsert?: boolean;
    continueOnError?: boolean;
    target?: 'runtime';
  };
}

/**
 * Per-document import outcome.
 */
export interface ImportItemOutcome {
  resourceType: string;
  resourceId?: string;
  resourceName?: string;
  operation?: 'create' | 'update';
  status: 'success' | 'failed';
  code?: string;
  message?: string;
}

/**
 * Import response payload from /import endpoint.
 */
export interface ImportResponse {
  summary: {
    totalDocuments: number;
    imported: number;
    failed: number;
    importedAt: string;
  };
  results: ImportItemOutcome[];
}
