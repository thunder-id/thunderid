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
import {describe, it, expect, vi} from 'vitest';
import IconGridPicker from '../IconGridPicker';

const ITEMS = Array.from({length: 30}, (_, i) => `icon-${i}`);

function iconsFor(names: string[]): Record<string, string> {
  return Object.fromEntries(names.map((name) => [name, `data:image/svg+xml,${name}`]));
}

describe('IconGridPicker', () => {
  it('should render exactly optionCount icons', () => {
    render(<IconGridPicker icons={iconsFor(ITEMS)} value="" shape="rounded" optionCount={5} onChange={vi.fn()} />);

    expect(screen.getAllByRole('img')).toHaveLength(5);
  });

  it('should include the selected icon among the rendered options even with a small sample', () => {
    render(
      <IconGridPicker icons={iconsFor(ITEMS)} value="icon-27" shape="rounded" optionCount={3} onChange={vi.fn()} />,
    );

    expect(screen.getByTitle('icon-27')).toBeInTheDocument();
  });
});
