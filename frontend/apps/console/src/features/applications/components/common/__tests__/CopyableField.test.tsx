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
import {describe, it, expect, beforeEach, vi} from 'vitest';
import CopyableField from '../CopyableField';

const mockCopy = vi.fn().mockResolvedValue(undefined);

vi.mock('@thunderid/hooks', () => ({
  useCopyToClipboard: vi.fn(() => ({copied: false, copy: mockCopy})),
}));

describe('CopyableField', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCopy.mockResolvedValue(undefined);
  });

  it('renders a read-only field with the given label and value', () => {
    render(<CopyableField id="field-id" label="Client ID" value="my-client-id" copyAriaLabel="Copy Client ID" />);

    expect(screen.getByText('Client ID')).toBeInTheDocument();
    const input = screen.getByDisplayValue('my-client-id');
    expect(input).toHaveAttribute('readonly');
  });

  it('copies the value when the copy button is clicked', async () => {
    const user = userEvent.setup();
    render(<CopyableField id="field-id" label="Client ID" value="my-client-id" copyAriaLabel="Copy Client ID" />);

    await user.click(screen.getByRole('button', {name: 'Copy Client ID'}));

    expect(mockCopy).toHaveBeenCalledWith('my-client-id');
  });
});
