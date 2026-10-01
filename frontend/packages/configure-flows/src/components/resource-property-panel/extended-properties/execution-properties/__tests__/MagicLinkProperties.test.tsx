// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import MagicLinkProperties from '../MagicLinkProperties';

// Mock react-i18next
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, defaultValue?: string) => {
      const translations: Record<string, string> = {
        'flows:core.executions.magicLink.description': 'Configure the Magic Link step behavior.',
        'flows:core.executions.magicLink.mode.label': 'Mode',
        'flows:core.executions.magicLink.mode.placeholder': 'Select an action mode',
        'flows:core.executions.magicLink.mode.generate': 'Generate Magic Link',
        'flows:core.executions.magicLink.mode.verify': 'Verify Magic Link',
      };
      return translations[key] ?? defaultValue ?? key;
    },
  }),
}));

describe('MagicLinkProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    mockOnChange.mockClear();
  });

  it('should render the description and mode label', () => {
    const resource = {
      data: {
        action: {
          executor: {
            mode: 'generate',
          },
        },
      },
    };

    render(<MagicLinkProperties resource={resource as unknown as Resource} onChange={mockOnChange} />);

    expect(screen.getByText('Configure the Magic Link step behavior.')).toBeInTheDocument();
    expect(screen.getByText('Mode')).toBeInTheDocument();
  });

  it('should display selected mode', () => {
    const resource = {
      data: {
        action: {
          executor: {
            mode: 'verify',
          },
        },
      },
    };

    render(<MagicLinkProperties resource={resource as unknown as Resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Verify Magic Link');
  });

  it('should display placeholder when no mode is selected', () => {
    const resource = {
      data: {
        action: {
          executor: {},
        },
      },
    };

    render(<MagicLinkProperties resource={resource as unknown as Resource} onChange={mockOnChange} />);

    expect(screen.getByRole('combobox')).toHaveTextContent('Select an action mode');
  });

  it('should call onChange with updated data when a new mode is selected', async () => {
    const user = userEvent.setup();
    const resource = {
      data: {
        action: {
          executor: {
            mode: 'generate',
          },
        },
        display: {
          label: 'Generate Magic Link',
        },
      },
    };

    render(<MagicLinkProperties resource={resource as unknown as Resource} onChange={mockOnChange} />);

    const selectButton = screen.getByRole('combobox');
    await user.click(selectButton);

    const verifyOption = screen.getByText('Verify Magic Link');
    await user.click(verifyOption);

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      {
        action: {
          executor: {
            mode: 'verify',
            inputs: [],
          },
        },
        display: {
          label: 'Verify Magic Link',
        },
      },
      resource,
    );
  });

  it('should default the display block when the node carries none', async () => {
    const user = userEvent.setup();
    const resource = {
      data: {
        action: {
          executor: {
            mode: 'generate',
          },
        },
      },
    };

    render(<MagicLinkProperties resource={resource as unknown as Resource} onChange={mockOnChange} />);

    const selectButton = screen.getByRole('combobox');
    await user.click(selectButton);

    const verifyOption = screen.getByText('Verify Magic Link');
    await user.click(verifyOption);

    expect(mockOnChange).toHaveBeenCalledWith(
      'data',
      expect.objectContaining({
        display: {label: 'Verify Magic Link'},
      }),
      resource,
    );
  });
});
