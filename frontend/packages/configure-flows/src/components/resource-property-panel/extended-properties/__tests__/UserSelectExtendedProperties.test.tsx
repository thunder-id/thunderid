// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import UserSelectExtendedProperties from '../UserSelectExtendedProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string | undefined) => value,
  }),
}));

describe('UserSelectExtendedProperties', () => {
  it('should have a Data source and a Submit value section, and no other', () => {
    const {container} = render(<UserSelectExtendedProperties />);

    expect(screen.getByText('flows:core.elements.userSelect.dataSource.title')).toBeInTheDocument();
    expect(screen.getByText('flows:core.elements.userSelect.submitValue.title')).toBeInTheDocument();
    expect(container.querySelectorAll('section')).toHaveLength(2);
  });

  describe('Data source', () => {
    it('should show the callback as fetchUsers(...) and nothing else about it', () => {
      render(<UserSelectExtendedProperties />);

      expect(screen.getByTestId('user-select-data-source')).toHaveTextContent(/^fetchUsers\(\.\.\.\)$/);
    });

    it('should show a short hint under the callback', () => {
      render(<UserSelectExtendedProperties />);

      expect(screen.getByText('flows:core.elements.userSelect.dataSource.hint')).toBeInTheDocument();
    });
  });

  describe('Submit value', () => {
    it('should show what the flow receives', () => {
      render(<UserSelectExtendedProperties />);

      expect(screen.getByTestId('user-select-submit-value')).toHaveTextContent(
        'flows:core.elements.userSelect.submitValue.value',
      );
    });

    it('should show a short hint under the value', () => {
      render(<UserSelectExtendedProperties />);

      expect(screen.getByText('flows:core.elements.userSelect.submitValue.hint')).toBeInTheDocument();
    });
  });

  it('should not offer anything to edit', () => {
    render(<UserSelectExtendedProperties />);

    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });
});
