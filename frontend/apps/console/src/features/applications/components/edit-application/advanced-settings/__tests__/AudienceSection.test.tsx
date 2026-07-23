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
import AudienceSection from '../AudienceSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string, opts?: {entity?: string}) =>
      (fallback ?? key).replace('{{entity}}', opts?.entity ?? ''),
  }),
}));

describe('AudienceSection', () => {
  const onAudienceChange = vi.fn();

  beforeEach(() => {
    onAudienceChange.mockClear();
  });

  it('renders the card and the current audience value', () => {
    render(<AudienceSection audience="https://api.example.com" onAudienceChange={onAudienceChange} />);

    expect(screen.getByText('Default Audience')).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://api.example.com')).toBeInTheDocument();
  });

  it('reports the typed audience (trimmed)', async () => {
    const user = userEvent.setup();
    render(<AudienceSection audience="" onAudienceChange={onAudienceChange} />);

    const input = screen.getByPlaceholderText('e.g. https://api.example.com');
    await user.type(input, 'x');

    expect(onAudienceChange).toHaveBeenLastCalledWith('x');
  });

  it('disables the input when disabled is set', () => {
    render(<AudienceSection audience="" onAudienceChange={onAudienceChange} disabled />);

    expect(screen.getByPlaceholderText('e.g. https://api.example.com')).toBeDisabled();
  });
});
