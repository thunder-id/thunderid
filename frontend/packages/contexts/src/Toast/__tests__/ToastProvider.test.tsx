// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, useEffect} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {beforeEach, afterEach, describe, expect, it, vi} from 'vitest';
import type {ToastSeverity} from '../ToastContext';
import ToastProvider from '../ToastProvider';
import useToast from '../useToast';

const DEFAULT_DURATION_MS = 6000;
const LONG_DURATION_MS = 12000;

// Long enough for the snackbar's exit transition to finish and unmount the node.
const EXIT_TRANSITION_MS = 300;

let container: HTMLDivElement;
let root: Root;
let showToast: (message: string, severity?: ToastSeverity, durationMs?: number) => void;

/**
 * Captures the context's showToast so tests can drive the provider the way callers do.
 */
function ToastTrigger() {
  const {showToast: contextShowToast} = useToast();

  useEffect(() => {
    showToast = contextShowToast;
  }, [contextShowToast]);

  return <span data-testid="child">child</span>;
}

function renderProvider(): void {
  act(() => {
    root.render(
      <ToastProvider>
        <ToastTrigger />
      </ToastProvider>,
    );
  });
}

function show(message: string, severity?: ToastSeverity, durationMs?: number): void {
  act(() => {
    showToast(message, severity, durationMs);
  });
}

function advance(ms: number): void {
  act(() => {
    vi.advanceTimersByTime(ms);
  });
}

// The snackbar renders through a portal, so it lives outside `container`.
function getAlert(): HTMLElement | null {
  return document.querySelector<HTMLElement>('.MuiAlert-root');
}

/**
 * The snackbar stays mounted while it transitions out, so presence alone does not mean
 * the toast is still up. Once auto-hide fires, the alert is faded to `opacity: 0`.
 */
function isToastVisible(): boolean {
  const alert = getAlert();
  return alert !== null && alert.style.opacity !== '0';
}

function getAlertText(): string {
  return document.querySelector('.MuiAlert-message')?.textContent ?? '';
}

beforeEach(() => {
  vi.useFakeTimers({toFake: ['setTimeout', 'clearTimeout']});
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  vi.useRealTimers();
});

describe('ToastProvider', () => {
  it('renders its children and shows no toast until one is requested', () => {
    renderProvider();

    expect(container.querySelector('[data-testid="child"]')?.textContent).toBe('child');
    expect(getAlert()).toBeNull();
  });

  it('shows the requested message', () => {
    renderProvider();
    show('Saved successfully.');

    expect(getAlertText()).toBe('Saved successfully.');
    expect(isToastVisible()).toBe(true);
  });

  it('defaults to the success severity', () => {
    renderProvider();
    show('Saved successfully.');

    expect(getAlert()?.className).toContain('MuiAlert-colorSuccess');
  });

  it('applies the requested severity', () => {
    renderProvider();
    show('Something went wrong.', 'error');

    expect(getAlert()?.className).toContain('MuiAlert-colorError');
  });

  it('replaces the current toast when a new one is requested', () => {
    renderProvider();
    show('Saved successfully.');
    show('Something went wrong.', 'error');

    expect(getAlertText()).toBe('Something went wrong.');
    expect(getAlert()?.className).toContain('MuiAlert-colorError');
  });

  it('auto hides after the default duration when none is given', () => {
    renderProvider();
    show('Saved successfully.');

    advance(DEFAULT_DURATION_MS - 1);
    expect(isToastVisible()).toBe(true);

    advance(1);
    expect(isToastVisible()).toBe(false);
  });

  it('unmounts the toast once it has transitioned out', () => {
    renderProvider();
    show('Saved successfully.');

    advance(DEFAULT_DURATION_MS);
    // The exit transition is only scheduled once the close has been applied.
    advance(EXIT_TRANSITION_MS);

    expect(getAlert()).toBeNull();
  });

  it('keeps a longer toast up past the default duration and hides it at the requested one', () => {
    renderProvider();
    show('A much longer error message that needs more reading time.', 'error', LONG_DURATION_MS);

    advance(DEFAULT_DURATION_MS);
    expect(isToastVisible()).toBe(true);

    advance(LONG_DURATION_MS - DEFAULT_DURATION_MS - 1);
    expect(isToastVisible()).toBe(true);

    advance(1);
    expect(isToastVisible()).toBe(false);
  });

  it('applies the duration of the most recent toast rather than the previous one', () => {
    renderProvider();
    show('Long lived error.', 'error', LONG_DURATION_MS);
    show('Short lived success.');

    expect(getAlertText()).toBe('Short lived success.');

    advance(DEFAULT_DURATION_MS);
    expect(isToastVisible()).toBe(false);
  });

  it('gives a toast shown while another is open its own full duration', () => {
    renderProvider();
    show('Application suspended.');

    advance(5000);
    show('Application unsuspended.');

    advance(DEFAULT_DURATION_MS - 1);
    expect(getAlertText()).toBe('Application unsuspended.');
    expect(isToastVisible()).toBe(true);

    advance(1);
    expect(isToastVisible()).toBe(false);
  });

  it('dismisses the toast when the close button is clicked', () => {
    renderProvider();
    show('Saved successfully.');

    const closeButton = document.querySelector<HTMLButtonElement>('.MuiAlert-action button');
    expect(closeButton).not.toBeNull();

    act(() => {
      closeButton?.click();
    });

    expect(isToastVisible()).toBe(false);
  });

  it('keeps the toast up when the user clicks away from it', () => {
    renderProvider();
    show('Saved successfully.');

    // The click-away listener activates on a zero-delay timer, so let that settle first.
    advance(1);

    act(() => {
      document.body.dispatchEvent(new MouseEvent('click', {bubbles: true}));
    });

    expect(isToastVisible()).toBe(true);
  });

  it('throws when useToast is used outside a provider', () => {
    expect(() => {
      act(() => {
        root.render(<ToastTrigger />);
      });
    }).toThrow('useToast must be used within a ToastProvider');
  });
});
