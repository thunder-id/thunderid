// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import {ElementCategories, ElementTypes, type Element} from '../../../../../models/elements';
import DynamicInputPlaceholderAdapter from '../DynamicInputPlaceholderAdapter';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.placeholders.dynamicInputPlaceholder.title': 'Dynamic Input',
        'flows:core.placeholders.dynamicInputPlaceholder.hint': 'Resolves input fields passed from runtime.',
      };

      return translations[key] ?? key;
    },
  }),
}));

describe('DynamicInputPlaceholderAdapter', () => {
  const createResource = (overrides: Partial<Element> = {}): Element =>
    ({
      id: 'dynamic-inputs',
      type: ElementTypes.DynamicInputPlaceholder,
      category: ElementCategories.Display,
      resourceType: 'ELEMENT',
      config: {},
      ...overrides,
    }) as Element;

  it('should render fallback translation copy when placeholder and hint are absent', () => {
    render(<DynamicInputPlaceholderAdapter resource={createResource()} />);

    expect(screen.getByText('Dynamic Input')).toBeInTheDocument();
    expect(screen.getByText('Resolves input fields passed from runtime.')).toBeInTheDocument();
  });

  it('should render custom placeholder and hint when present on the resource', () => {
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
