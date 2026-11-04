// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

  describe('masked', () => {
    const maskedComponent: FlowComponent = {...baseComponent, label: 'Client Secret', masked: true};

    const secretInput = (): HTMLInputElement => screen.getByTestId('copyable-1').querySelector('input')!;

    beforeEach(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn().mockResolvedValue(undefined)},
        configurable: true,
        writable: true,
      });
    });

    it('renders the label and a read-only password field holding the value', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      expect(screen.getByText('Client Secret')).toBeTruthy();
      expect(secretInput().type).toBe('password');
      expect(secretInput().value).toBe('https://example.com/invite/abc123');
      expect(secretInput().readOnly).toBe(true);
    });

    it('does not show the value as text', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      expect(screen.queryByText('https://example.com/invite/abc123')).toBeNull();
    });

    it('keeps the field after the label, where the plain layout puts the value box', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      const label = screen.getByText('Client Secret');
      expect(label.nextElementSibling?.tagName).toBe('DIV');
      expect(label.nextElementSibling?.querySelector('input')).toBe(secretInput());
    });

    it('names the field after its label for assistive technology', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      expect(secretInput().getAttribute('aria-label')).toBe('Client Secret');
      expect(secretInput().getAttribute('autocomplete')).toBe('off');
    });

    it('reveals the value when Show is pressed', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.show'}));

      expect(secretInput().type).toBe('text');
    });

    it('hides the value again when Hide is pressed', () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      fireEvent.click(screen.getByRole('button', {name: 'actions.show'}));

      fireEvent.click(screen.getByRole('button', {name: 'actions.hide'}));

      expect(secretInput().type).toBe('password');
    });

    it('copies the real value while it is hidden', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('https://example.com/invite/abc123');
      });
      expect(secretInput().type).toBe('password');
    });

    it('shows copied feedback on the copy control', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'actions.copied'})).toBeTruthy();
      });
    });

    it('renders an empty field when the source key is not in additionalData', () => {
      renderWithProviders(
        <CopyableTextAdapter component={{...maskedComponent, source: 'missingKey'}} resolve={(s) => s} />,
      );

      expect(secretInput().value).toBe('');
    });

    it('falls back to execCommand when the clipboard API throws', async () => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn().mockRejectedValue(new Error('Not allowed'))},
        configurable: true,
        writable: true,
      });
      const execCommandSpy = vi.spyOn(document, 'execCommand').mockReturnValue(true);
      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(execCommandSpy).toHaveBeenCalledWith('copy');
      });
    });

    it('is not used when the component is not masked', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      expect(document.querySelector('input')).toBeNull();
      expect(screen.queryByRole('button', {name: 'actions.show'})).toBeNull();
    });
  });

  describe('copy control', () => {
    const maskedComponent: FlowComponent = {...baseComponent, label: 'Client Secret', masked: true};

    it('is an icon button with no visible text in the plain layout', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      const copy = screen.getByRole('button', {name: 'actions.copy'});

      expect(copy.textContent).toBe('');
      expect(copy.querySelector('svg')).not.toBeNull();
    });

    it('sits in the same box as the value in the plain layout', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      const copy = screen.getByRole('button', {name: 'actions.copy'});
      const value = screen.getByText('https://example.com/invite/abc123');

      expect(copy.closest('div')).toBe(value.closest('div'));
    });

    it('uses the same icon in the plain and the masked layouts', () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      const plainIcon = screen.getByRole('button', {name: 'actions.copy'}).querySelector('svg')?.innerHTML;
      cleanup();

      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      const maskedIcon = screen.getByRole('button', {name: 'actions.copy'}).querySelector('svg')?.innerHTML;

      expect(plainIcon).toBeTruthy();
      expect(maskedIcon).toBe(plainIcon);
    });

    it('swaps to the same check icon after copying in both layouts', async () => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn().mockResolvedValue(undefined)},
        configurable: true,
        writable: true,
      });
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));
      const plainCheck = (await screen.findByRole('button', {name: 'actions.copied'})).querySelector('svg')?.innerHTML;
      cleanup();

      renderWithProviders(
        <CopyableTextAdapter component={maskedComponent} resolve={(s) => s} additionalData={additionalData} />,
      );
      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));
      const maskedCheck = (await screen.findByRole('button', {name: 'actions.copied'})).querySelector('svg')?.innerHTML;

      expect(plainCheck).toBeTruthy();
      expect(maskedCheck).toBe(plainCheck);
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

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(navigator.clipboard.writeText).toHaveBeenCalledWith('https://example.com/invite/abc123');
      });
    });

    it('shows "Copied!" feedback after clicking Copy', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'actions.copied'})).toBeTruthy();
      });
    });

    it('resets back to "Copy" label after 3 seconds', async () => {
      renderWithProviders(
        <CopyableTextAdapter component={baseComponent} resolve={(s) => s} additionalData={additionalData} />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(screen.getByRole('button', {name: 'actions.copied'})).toBeTruthy();
      });

      await waitFor(
        () => {
          expect(screen.getByRole('button', {name: 'actions.copy'})).toBeTruthy();
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

      fireEvent.click(screen.getByRole('button', {name: 'actions.copy'}));

      await waitFor(() => {
        expect(execCommandSpy).toHaveBeenCalledWith('copy');
      });
    });
  });
});
