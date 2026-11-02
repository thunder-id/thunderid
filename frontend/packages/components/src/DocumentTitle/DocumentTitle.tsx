// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import type {JSX} from 'react';

export interface DocumentTitleProps {
  /** The page or resource name. When omitted, only the product name is shown. */
  title?: string;
}

/**
 * Sets the browser tab title as `[page or resource] | [product name]`, using the product name
 * from the runtime brand configuration. React hoists the rendered `<title>` into the document head.
 */
export default function DocumentTitle({title = undefined}: DocumentTitleProps): JSX.Element {
  const {config} = useConfig();
  const productName = config.brand.product_name;

  return <title>{title ? `${title} | ${productName}` : productName}</title>;
}
