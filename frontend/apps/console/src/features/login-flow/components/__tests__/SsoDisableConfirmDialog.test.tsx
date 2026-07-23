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

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SsoDisableConfirmDialog from '../SsoDisableConfirmDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: unknown) => {
      if (typeof options === 'string') {
        return options;
      }
      return key;
    },
  }),
}));

describe('SsoDisableConfirmDialog', () => {
  const mockOnClose = vi.fn();
  const mockOnConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should render the dialog when open', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Remove single sign-on?')).toBeInTheDocument();
  });

  it('should not render dialog content when closed', () => {
    render(
      <SsoDisableConfirmDialog open={false} checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />,
    );

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('should call onConfirm when the remove button is clicked', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={2} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByTestId('sso-disable-confirm-button'));

    expect(mockOnConfirm).toHaveBeenCalledTimes(1);
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it('should call onClose when the cancel button is clicked', () => {
    render(<SsoDisableConfirmDialog open checkpointCount={1} onClose={mockOnClose} onConfirm={mockOnConfirm} />);

    fireEvent.click(screen.getByText('Cancel'));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
    expect(mockOnConfirm).not.toHaveBeenCalled();
  });
});
