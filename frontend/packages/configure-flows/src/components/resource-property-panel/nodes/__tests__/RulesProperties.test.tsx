// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import RulesProperties from '../RulesProperties';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

describe('RulesProperties', () => {
  describe('Rendering', () => {
    it('should render the component', () => {
      render(<RulesProperties />);

      expect(screen.getByText('flows:core.rulesProperties.description')).toBeInTheDocument();
    });

    it('should render Typography with body2 variant', () => {
      const {container} = render(<RulesProperties />);

      const typography = container.querySelector('.MuiTypography-body2');
      expect(typography).toBeInTheDocument();
    });

    it('should render within a Stack component', () => {
      const {container} = render(<RulesProperties />);

      const stack = container.querySelector('.MuiStack-root');
      expect(stack).toBeInTheDocument();
    });
  });
});
