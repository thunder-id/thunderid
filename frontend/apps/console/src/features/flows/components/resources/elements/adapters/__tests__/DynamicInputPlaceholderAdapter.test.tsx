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

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import type {Element} from '@/features/flows/models/elements';
import {ElementCategories, ElementTypes} from '@/features/flows/models/elements';
import DynamicInputPlaceholderAdapter from '../DynamicInputPlaceholderAdapter';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, fallback?: string) => fallback ?? _key,
  }),
}));

describe('DynamicInputPlaceholderAdapter', () => {
  const createResource = (overrides: Partial<Element> = {}): Element =>
    ({
      id: 'dynamic-input-placeholder',
      type: ElementTypes.DynamicInputPlaceholder,
      category: ElementCategories.Display,
      config: {},
      ...overrides,
    }) as Element;

  it('should render translated fallback copy when placeholder and hint are absent', () => {
    render(<DynamicInputPlaceholderAdapter resource={createResource()} />);

    expect(screen.getByText('Dynamic Input')).toBeInTheDocument();
    expect(screen.getByText('Resolves input fields passed from runtime when the flow executes')).toBeInTheDocument();
  });

  it('should prefer resource placeholder and hint values when provided', () => {
    render(
      <DynamicInputPlaceholderAdapter
        resource={createResource({
          hint: 'Custom hint',
          placeholder: 'Custom placeholder',
        } as Partial<Element>)}
      />,
    );

    expect(screen.getByText('Custom placeholder')).toBeInTheDocument();
    expect(screen.getByText('Custom hint')).toBeInTheDocument();
  });
});
