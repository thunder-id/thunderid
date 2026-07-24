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
import DiscardChangesDialog from '../DiscardChangesDialog';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => fallback ?? key}),
}));

describe('DiscardChangesDialog', () => {
  const onClose = vi.fn();
  const onConfirm = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders when open', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByText('Discard unsaved changes?')).toBeInTheDocument();
  });

  it('does not render content when closed', () => {
    render(<DiscardChangesDialog open={false} onClose={onClose} onConfirm={onConfirm} />);

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('calls onConfirm when the discard button is clicked', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    fireEvent.click(screen.getByTestId('discard-changes-confirm-button'));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onClose).not.toHaveBeenCalled();
  });

  it('calls onClose when the keep-editing button is clicked', () => {
    render(<DiscardChangesDialog open onClose={onClose} onConfirm={onConfirm} />);

    fireEvent.click(screen.getByText('Keep editing'));

    expect(onClose).toHaveBeenCalledTimes(1);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
