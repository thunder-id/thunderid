// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import NoConfigProperties from '../NoConfigProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.noConfig.description': 'No configurable properties for this executor.',
      };
      return translations[key] || key;
    },
  }),
}));

describe('NoConfigProperties', () => {
  it('should render no config description', () => {
    render(<NoConfigProperties />);

    expect(screen.getByText('No configurable properties for this executor.')).toBeInTheDocument();
  });
});
