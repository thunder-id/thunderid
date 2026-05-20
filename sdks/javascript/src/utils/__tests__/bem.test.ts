/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {describe, it, expect} from 'vitest';
import bem from '../bem';

describe('bem', () => {
  it('should return base class when only baseClass is provided', () => {
    expect(bem('btn')).toBe('btn');
  });

  it('should append element when provided', () => {
    expect(bem('btn', 'icon')).toBe('btn__icon');
  });

  it('should append modifier when provided', () => {
    expect(bem('btn', null, 'primary')).toBe('btn--primary');
  });

  it('should append element and modifier when both are provided', () => {
    expect(bem('btn', 'icon', 'small')).toBe('btn__icon--small');
  });

  it('should ignore undefined / null element', () => {
    expect(bem('card', undefined, 'selected')).toBe('card--selected');
    expect(bem('card', null, 'selected')).toBe('card--selected');
  });

  it('should ignore undefined / null modifier', () => {
    expect(bem('card', 'header', undefined)).toBe('card__header');
    expect(bem('card', 'header', null)).toBe('card__header');
  });

  it('should treat empty string element/modifier as absent (no suffix added)', () => {
    expect(bem('chip', '', 'active' as unknown as string)).toBe('chip--active');
    expect(bem('chip', 'label', '' as unknown as string)).toBe('chip__label');
    expect(bem('chip', '', '' as unknown as string)).toBe('chip');
  });

  it('should pass through special characters in element and modifier', () => {
    expect(bem('block', 'el-1_2', 'mod-3_4')).toBe('block__el-1_2--mod-3_4');
    expect(bem('x', '🎯', '🔥')).toBe('x__🎯--🔥');
  });
});
