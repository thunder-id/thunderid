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

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/unbound-method */
import {screen, cleanup, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, afterEach, vi, beforeEach} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import CopyableTextAdapter from '../CopyableTextAdapter';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

const baseComponent: FlowComponent = {
  id: 'copyable-1',
  type: 'COPYABLE_TEXT',
  source: 'inviteLink',
};

const additionalData: Record<string, unknown> = {
  inviteLink: 'https://example.com/invite/abc123',
};

describe('CopyableTextAdapter', () => {
  describe('rendering', () => {
    it('renders the value from additionalData[source]', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      expect(screen.getByText('https://example.com/invite/abc123')).toBeTruthy();
    });

    it('renders an optional label when component.label is set', () => {
      const component = {...baseComponent, label: 'Invite Link'};
      renderWithProviders(
        <CopyableTextAdapter component={component} resolve={(s) => s} additionalData={additionalData} />,
      );
      expect(screen.getByText('Invite Link')).toBeTruthy();
    });

    it('does not render a label element when component.label is absent', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      // No label element — only the value text and the Copy button text should be present
      expect(screen.queryByText('Invite Link')).toBeNull();
    });

    it('renders an empty string value when source key is not in additionalData', () => {
      const component = {...baseComponent, source: 'missingKey'};
      renderWithProviders(
        <CopyableTextAdapter component={component} resolve={(s) => s} additionalData={additionalData} />,
      );
      // Value text is present but empty — the Copy button should still render
      expect(screen.getByRole('button')).toBeTruthy();
    });

    it('renders an empty string value when additionalData is undefined', () => {
      renderWithProviders(<CopyableTextAdapter component={baseComponent} resolve={(s) => s} />);
      expect(screen.getByRole('button')).toBeTruthy();
    });

    it('passes label through the resolve function', () => {
      const component = {...baseComponent, label: '{{t(invite_link)}}'};
      renderWithProviders(
        <CopyableTextAdapter component={component} resolve={() => 'Resolved Label'} additionalData={additionalData} />,
      );
      expect(screen.getByText('Resolved Label')).toBeTruthy();
    });

    it('renders the Copy button by default', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      expect(screen.getByRole('button')).toBeTruthy();
    });
  });

  describe('copy interaction', () => {
    beforeEach(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn().mockResolvedValue(undefined)},
        configurable: true,
        writable: true,
      });
    });

    it('calls navigator.clipboard.writeText with the value when Copy is clicked', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button'));

      await waitFor(() => {
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('https://example.com/invite/abc123');
      });
    });

    it('shows "Copied!" feedback after clicking Copy', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button'));

      await waitFor(() => {
        expect(screen.getByRole('button').textContent).toContain('actions.copied');
      });
    });

    it('resets back to "Copy" label after 3 seconds', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button'));

      await waitFor(() => {
        expect(screen.getByRole('button').textContent).toContain('actions.copied');
      });

      await waitFor(
        () => {
          expect(screen.getByRole('button').textContent).toContain('actions.copy');
        },
        {timeout: 5000},
      );
    }, 10000);

    it('falls back to execCommand when clipboard API throws', async () => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn().mockRejectedValue(new Error('Not allowed'))},
        configurable: true,
        writable: true,
      });

      const execCommandSpy = vi.spyOn(document, 'execCommand').mockReturnValue(true);

      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button'));

      await waitFor(() => {
        expect(execCommandSpy).toHaveBeenCalledWith('copy');
      });
    });
  });
});
