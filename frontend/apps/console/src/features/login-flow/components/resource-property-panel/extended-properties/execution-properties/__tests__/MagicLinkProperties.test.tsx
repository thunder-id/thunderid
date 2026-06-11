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
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import MagicLinkProperties from '../MagicLinkProperties';
import type {Resource} from '@/features/flows/models/resources';

// Mock react-i18next
vi.mock('react-i18next', () => ({
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
});
