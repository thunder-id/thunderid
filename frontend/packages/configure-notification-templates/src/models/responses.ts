// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TemplateSummary} from './notification-template';

/**
 * A HATEOAS-style link returned alongside a template listing.
 *
 * @public
 */
export interface TemplateLink {
  rel?: string;
  href?: string;
}

/**
 * Response structure for the paginated notification-template list endpoint.
 *
 * This is the response from `GET /notification-templates/{channel}/templates`.
 *
 * @public
 */
export interface TemplateListResponse {
  /**
   * Total number of templates available for the channel.
   */
  totalResults: number;
  /**
   * Zero-based index of the first template in the current response.
   */
  startIndex: number;
  /**
   * Number of templates in the current response.
   */
  count: number;
  /**
   * The templates in the current page.
   */
  templates: TemplateSummary[];
  /**
   * Pagination links.
   */
  links?: TemplateLink[];
}
