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

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ConfigureSeparator from '../ConfigureSeparator';

describe('ConfigureSeparator', () => {
  it('renders the separator select element', () => {
    render(<ConfigureSeparator delimiter=":" handle="my-api" onDelimiterChange={vi.fn()} />);

    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('renders the permission preview using the provided handle', () => {
    render(<ConfigureSeparator delimiter=":" handle="payments-api" onDelimiterChange={vi.fn()} />);

    expect(screen.getByText('payments-api:<resource>:<action>')).toBeInTheDocument();
  });

  it('uses "my-api" as fallback handle in preview when handle is empty', () => {
    render(<ConfigureSeparator delimiter=":" handle="" onDelimiterChange={vi.fn()} />);

    expect(screen.getByText('my-api:<resource>:<action>')).toBeInTheDocument();
  });

  it('renders preview using dot separator', () => {
    render(<ConfigureSeparator delimiter="." handle="my-api" onDelimiterChange={vi.fn()} />);

    expect(screen.getByText('my-api.<resource>.<action>')).toBeInTheDocument();
  });

  it('calls onReadyChange with true when delimiter is valid', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureSeparator delimiter=":" handle="my-api" onDelimiterChange={vi.fn()} onReadyChange={onReadyChange} />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });
});
