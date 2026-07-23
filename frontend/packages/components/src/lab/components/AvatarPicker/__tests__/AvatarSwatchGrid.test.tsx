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

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-call */
import {render, screen} from '@testing-library/react';
import {generateAvatarDataUri} from '@thunderid/react';
import {describe, it, expect, vi} from 'vitest';
import AvatarSwatchGrid from '../AvatarSwatchGrid';

describe('AvatarSwatchGrid', () => {
  const base = {content: '', shape: 'rounded' as const, variant: 'blank' as const};

  it('should render exactly optionCount swatches', () => {
    render(
      <AvatarSwatchGrid
        base={base}
        value={-1}
        gradientCount={20}
        optionCount={5}
        showShuffle={false}
        onChange={vi.fn()}
      />,
    );

    expect(screen.getAllByRole('button')).toHaveLength(5);
  });

  it('should include the selected swatch among the rendered options even with a small sample', () => {
    const {container} = render(
      <AvatarSwatchGrid base={base} value={17} gradientCount={40} optionCount={3} onChange={vi.fn()} />,
    );

    const expectedSrc = generateAvatarDataUri({...base, colors: 17});
    const images = Array.from(container.querySelectorAll('img'));
    expect(images.some((img) => img.getAttribute('src') === expectedSrc)).toBe(true);
  });
});
