// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AppWithDecorators from '../AppWithDecorators';

// Mock HOCs as pass-through so tests focus on composition
vi.mock('../hocs/withConfig', () => ({
  default: (Component: React.ComponentType) => Component,
}));
vi.mock('../hocs/withDesign', () => ({
  default: (Component: React.ComponentType) => Component,
}));
vi.mock('../hocs/withI18n', () => ({
  default: (Component: React.ComponentType) => Component,
}));
vi.mock('../hocs/withTheme', () => ({
  default: (Component: React.ComponentType) => Component,
}));

// Mock App
vi.mock('../App', () => ({
  default: () => <div data-testid="app">App</div>,
}));

// Mock i18next (used at module level in withI18n even though HOC is mocked)
vi.mock('i18next', () => ({
  default: {
    use: vi.fn().mockReturnThis(),
    init: vi.fn().mockResolvedValue(undefined),
  },
}));
vi.mock('react-i18next', () => ({
  initReactI18next: {},
}));
vi.mock('@thunderid/i18n/locales/en-US', () => ({
  default: {common: {}, navigation: {}},
}));

describe('AppWithDecorators (admin-console)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders without crashing', () => {
    const {container} = render(<AppWithDecorators />);
    expect(container).toBeInTheDocument();
  });

  it('renders the App component', () => {
    render(<AppWithDecorators />);
    expect(screen.getByTestId('app')).toBeInTheDocument();
  });
});
