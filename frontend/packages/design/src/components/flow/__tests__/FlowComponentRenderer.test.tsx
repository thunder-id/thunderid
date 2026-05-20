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
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {screen, cleanup} from '@testing-library/react';
import {describe, it, expect, afterEach, vi} from 'vitest';
import renderWithProviders from '../../../test/renderWithProviders';
import FlowComponentRenderer from '../FlowComponentRenderer';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const noop = () => undefined;
const identity = (s: string | undefined) => s;

describe('FlowComponentRenderer — COPYABLE_TEXT routing', () => {
  it('renders CopyableTextAdapter when component type is COPYABLE_TEXT', () => {
    const component = {
      id: 'copyable-1',
      type: 'COPYABLE_TEXT',
      source: 'inviteLink',
      label: 'Invite Link',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/abc123'}}
      />,
    );

    expect(screen.getByText('https://example.com/invite/abc123')).toBeTruthy();
  });

  it('renders the label from COPYABLE_TEXT component', () => {
    const component = {
      id: 'copyable-2',
      type: 'COPYABLE_TEXT',
      source: 'inviteLink',
      label: 'Invite Link',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/xyz'}}
      />,
    );

    expect(screen.getByText('Invite Link')).toBeTruthy();
  });

  it('renders CopyableTextAdapter with empty value when source key is absent from additionalData', () => {
    const component = {
      id: 'copyable-3',
      type: 'COPYABLE_TEXT',
      source: 'missingKey',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/abc123'}}
      />,
    );

    // CopyableTextAdapter still renders with empty value — Copy button present
    expect(screen.getByRole('button')).toBeTruthy();
  });

  it('returns null for an unknown component type', () => {
    const component = {
      id: 'unknown-1',
      type: 'UNKNOWN_TYPE',
    } as unknown as EmbeddedFlowComponent;

    const {container} = renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect(container.firstChild).toBeNull();
  });
});
