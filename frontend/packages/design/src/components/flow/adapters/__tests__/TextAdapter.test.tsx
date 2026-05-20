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

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup} from '@testing-library/react';
import {describe, it, expect, afterEach} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import TextAdapter from '../TextAdapter';

afterEach(() => {
  cleanup();
});

const baseComponent: FlowComponent = {
  id: 'text-1',
  type: 'TEXT',
  label: 'Hello World',
};

describe('TextAdapter', () => {
  it('renders the resolved label', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />);
    expect(screen.getByText('Hello World')).toBeTruthy();
  });

  it('passes resolved label through the resolve function', () => {
    const component = {...baseComponent, label: '{{t(greet)}}'};
    renderWithProviders(<TextAdapter component={component} resolve={() => 'Resolved Text'} />);
    expect(screen.getByText('Resolved Text')).toBeTruthy();
  });

  it('applies product prefix CSS class names', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />);
    const el = screen.getByText('Hello World');
    expect(el.className).toContain('ThunderIDFlow--text');
  });

  it('uses center alignment when design mode is enabled and no align prop', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: true},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('center');
  });

  it('uses component.align when provided, overriding design mode', () => {
    const component = {...baseComponent, align: 'center'};
    renderWithProviders(<TextAdapter component={component} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: false},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('center');
  });

  it('falls back to left alignment when no align and design mode is disabled', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: false},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('left');
  });
});
