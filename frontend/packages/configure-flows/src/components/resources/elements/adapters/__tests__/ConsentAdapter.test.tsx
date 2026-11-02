// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import ConsentAdapter from '../ConsentAdapter';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback: string) => fallback ?? key,
  }),
}));

describe('ConsentAdapter', () => {
  it('should render the placeholder text', () => {
    render(<ConsentAdapter />);

    expect(screen.getByText('Consent attributes will appear here at runtime')).toBeInTheDocument();
  });

  it('should render italic styled typography', () => {
    const {container} = render(<ConsentAdapter />);

    const typography = container.querySelector('.MuiTypography-root');
    expect(typography).toBeInTheDocument();
    expect(typography).toHaveStyle({fontStyle: 'italic'});
  });
});
