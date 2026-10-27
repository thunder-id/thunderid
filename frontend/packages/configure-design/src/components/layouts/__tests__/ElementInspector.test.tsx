// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, cleanup, fireEvent, act} from '@testing-library/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {setCnPrefix} from '@thunderid/utils';
import {describe, it, expect, vi, afterEach, beforeEach} from 'vitest';
import ElementInspector from '../ElementInspector';

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');
  return {
    ...actual,
  };
});

const mockWriteText = vi.fn().mockResolvedValue(undefined);

beforeEach(() => {
  setCnPrefix(TEST_CN_PREFIX);
  Object.defineProperty(navigator, 'clipboard', {
    value: {writeText: mockWriteText},
    writable: true,
    configurable: true,
  });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  mockWriteText.mockReset().mockResolvedValue(undefined);
});

describe('ElementInspector', () => {
  describe('rendering', () => {
    it('renders children', () => {
      render(
        <ElementInspector enabled={false}>
          <span>Child content</span>
        </ElementInspector>,
      );
      expect(screen.getByText('Child content')).toBeTruthy();
    });

    it('renders with crosshair cursor when enabled', () => {
      const {container} = render(
        <ElementInspector enabled>
          <span>Content</span>
        </ElementInspector>,
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper).toBeTruthy();
    });

    it('renders with default cursor when disabled', () => {
      const {container} = render(
        <ElementInspector enabled={false}>
          <span>Content</span>
        </ElementInspector>,
      );
      const wrapper = container.firstElementChild as HTMLElement;
      expect(wrapper).toBeTruthy();
    });
  });

  describe('hover inspection', () => {
    it('shows highlight overlay when hovering over a prefixed classed element', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>Sign in</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Sign in');

      // Mock getBoundingClientRect for the target and container
      const containerEl = container.firstElementChild as HTMLElement;
      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });

      // The tooltip should show the tag and class
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}SignInBox--root`))).toBeTruthy();
    });

    it('walks up DOM tree to find nearest product name prefixed classed element', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>
            <span data-testid="inner">Inner text</span>
          </div>
        </ElementInspector>,
      );

      const innerEl = screen.getByTestId('inner');
      const productNamePrefixedEl = innerEl.parentElement!;
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(productNamePrefixedEl, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(innerEl);
      });

      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}SignInBox--root`))).toBeTruthy();
    });

    it('clears highlight when mouse leaves the container', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>Sign in</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Sign in');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}SignInBox--root`))).toBeTruthy();

      // Mouse out to an element outside the container
      act(() => {
        fireEvent.mouseOut(target, {relatedTarget: document.body});
      });
      expect(screen.queryByText(new RegExp(`${TEST_CN_PREFIX}SignInBox--root`))).toBeNull();
    });

    it('does not clear highlight when mouse moves between elements inside container', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>
            <span data-testid="a">A</span>
            <span className={`${TEST_CN_PREFIX}Flow--text`} data-testid="b">
              B
            </span>
          </div>
        </ElementInspector>,
      );

      const containerEl = container.firstElementChild as HTMLElement;
      const elA = screen.getByTestId('a');
      const elB = screen.getByTestId('b');
      const root = elA.parentElement!;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(root, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });
      vi.spyOn(elB, 'getBoundingClientRect').mockReturnValue({
        x: 50,
        y: 20,
        width: 100,
        height: 50,
        top: 20,
        left: 50,
        right: 150,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(elA);
      });

      // mouseout from A to B (both inside container) - should NOT clear
      act(() => {
        fireEvent.mouseOut(elA, {relatedTarget: elB});
      });

      // Some highlight should still be showing
      // The tooltip updates on mouseOver of B
      act(() => {
        fireEvent.mouseOver(elB);
      });
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();
    });

    it('clears highlight when pointer moves onto empty container space', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>Sign in</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Sign in');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      // First hover over a product name prefixed element to show highlight
      act(() => {
        fireEvent.mouseOver(target);
      });
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}SignInBox--root`))).toBeTruthy();

      // Then move onto the container's empty space — highlight should clear
      act(() => {
        fireEvent.mouseOver(containerEl);
      });
      expect(screen.queryByText(new RegExp(`\\.${TEST_CN_PREFIX}SignInBox--root`))).toBeNull();
    });

    it('sets highlight to null when walk-up finds no product name prefixed classes', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div data-testid="plain">No special classes here</div>
        </ElementInspector>,
      );

      const target = screen.getByTestId('plain');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });

      // No tooltip should appear since there are no product name prefixed classes — look for the dot-prefixed class pattern
      expect(screen.queryByText(new RegExp(`\\.${TEST_CN_PREFIX}`))).toBeNull();
    });
  });

  describe('click to select', () => {
    it('calls onSelectSelector with the best product name prefixed class on click', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      expect(onSelect).toHaveBeenCalledWith(`.${TEST_CN_PREFIX}SignInBox--root`);
    });

    it('prefers BEM modifier classes (with --) when picking best class', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div className={`${TEST_CN_PREFIX}Flow ${TEST_CN_PREFIX}Flow--button`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      expect(onSelect).toHaveBeenCalledWith(`.${TEST_CN_PREFIX}Flow--button`);
    });

    it('copies selector to clipboard on click', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      expect(mockWriteText).toHaveBeenCalledWith(`.${TEST_CN_PREFIX}Flow--text`);
    });

    it('shows copied feedback after click', async () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      await act(async () => {
        fireEvent.click(target);
        // Let the clipboard promise resolve
        await Promise.resolve();
      });

      expect(screen.getByText(new RegExp(`Copied:.*\\.${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();
    });

    it('clears copied feedback after timeout', async () => {
      vi.useFakeTimers({shouldAdvanceTime: true});

      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      await act(async () => {
        fireEvent.click(target);
        await Promise.resolve();
      });

      expect(screen.getByText(new RegExp(`Copied:.*\\.${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();

      // Advance past the 1500ms timeout
      act(() => {
        vi.advanceTimersByTime(1600);
      });

      expect(screen.queryByText(/Copied:/)).toBeNull();

      vi.useRealTimers();
    });

    it('shows copied feedback even when clipboard fails', async () => {
      mockWriteText.mockRejectedValue(new Error('Clipboard failed'));

      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Click me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Click me');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      await act(async () => {
        fireEvent.click(target);
        // Let the clipboard promise reject
        await Promise.resolve();
        await Promise.resolve();
      });

      expect(screen.getByText(new RegExp(`Copied:.*\\.${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();
    });

    it('does not call onSelectSelector when clicking element with no product name prefixed classes', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div data-testid="plain">No classes</div>
        </ElementInspector>,
      );

      const target = screen.getByTestId('plain');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      expect(onSelect).not.toHaveBeenCalled();
    });

    it('walks up to find product name prefixed parent when clicking a child', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div className={`${TEST_CN_PREFIX}SignInBox--root`}>
            <span data-testid="inner-span">Click inner</span>
          </div>
        </ElementInspector>,
      );

      const target = screen.getByTestId('inner-span');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      expect(onSelect).toHaveBeenCalledWith(`.${TEST_CN_PREFIX}SignInBox--root`);
    });
  });

  describe('disabled state', () => {
    it('does not show highlight when disabled', () => {
      render(
        <ElementInspector enabled={false}>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Hover me</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Hover me');

      act(() => {
        fireEvent.mouseOver(target);
      });

      expect(screen.queryByText(new RegExp(`${TEST_CN_PREFIX}Flow--text`))).toBeNull();
    });

    it('clears highlight and copied text when toggled from enabled to disabled', () => {
      const {container, rerender} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Content');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();

      // Disable the inspector
      rerender(
        <ElementInspector enabled={false}>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      expect(screen.queryByText((t) => t.includes(`${TEST_CN_PREFIX}Flow--text`) && !t.includes('Content'))).toBeNull();
    });
  });

  describe('tooltip display', () => {
    it('shows tag name in tooltip', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Content');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });

      expect(screen.getByText('<div>')).toBeTruthy();
    });

    it('shows "no classes" when element has no product name prefixed classes but is the walk-up target', () => {
      // This scenario happens when the element IS the container child (no parent with product name prefixed classes)
      // The walk-up stops at container, so no highlight is shown
      // But if an element has product name prefixed classes in its class list from the walk-up, classes are shown
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text ${TEST_CN_PREFIX}Flow--heading`}>Multi-class</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Multi-class');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });

      // Multiple classes should be shown joined with dots
      expect(screen.getByText(new RegExp(`\\.${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();
      expect(screen.getByText(new RegExp(`\\.${TEST_CN_PREFIX}Flow--heading`))).toBeTruthy();
    });
  });

  describe('mouseOut edge cases', () => {
    it('clears highlight when relatedTarget is null (mouse left the window)', () => {
      const {container} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Content');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });
      vi.spyOn(target, 'getBoundingClientRect').mockReturnValue({
        x: 10,
        y: 20,
        width: 200,
        height: 50,
        top: 20,
        left: 10,
        right: 210,
        bottom: 70,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.mouseOver(target);
      });
      expect(screen.getByText(new RegExp(`${TEST_CN_PREFIX}Flow--text`))).toBeTruthy();

      // mouseOut with null relatedTarget (pointer left the browser window)
      act(() => {
        fireEvent.mouseOut(target, {relatedTarget: null});
      });
      expect(screen.queryByText(new RegExp(`\\.${TEST_CN_PREFIX}Flow--text`))).toBeNull();
    });
  });

  describe('click edge cases', () => {
    it('does nothing when clicking on the container element itself', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const containerEl = container.firstElementChild as HTMLElement;

      act(() => {
        fireEvent.click(containerEl);
      });

      expect(onSelect).not.toHaveBeenCalled();
      expect(mockWriteText).not.toHaveBeenCalled();
    });
  });

  describe('cleanup', () => {
    it('removes event listeners when enabled changes to false', () => {
      const {container, rerender} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const containerEl = container.firstElementChild as HTMLElement;
      const removeSpy = vi.spyOn(containerEl, 'removeEventListener');

      rerender(
        <ElementInspector enabled={false}>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      // The cleanup should have called removeEventListener for mouseover, mouseout, and click
      const removedEvents = removeSpy.mock.calls.map((call) => call[0]);
      expect(removedEvents).toContain('mouseover');
      expect(removedEvents).toContain('mouseout');
      expect(removedEvents).toContain('click');
    });

    it('removes event listeners on unmount', () => {
      const {container, unmount} = render(
        <ElementInspector enabled>
          <div className={`${TEST_CN_PREFIX}Flow--text`}>Content</div>
        </ElementInspector>,
      );

      const containerEl = container.firstElementChild as HTMLElement;
      const removeSpy = vi.spyOn(containerEl, 'removeEventListener');

      unmount();

      const removedEvents = removeSpy.mock.calls.map((call) => call[0]);
      expect(removedEvents).toContain('mouseover');
      expect(removedEvents).toContain('mouseout');
      expect(removedEvents).toContain('click');
    });
  });

  describe('pickBestClass fallback', () => {
    it('falls back to first product name prefixed class when none contain --', () => {
      const onSelect = vi.fn();
      const {container} = render(
        <ElementInspector enabled onSelectSelector={onSelect}>
          <div className={`${TEST_CN_PREFIX}Base`}>Fallback</div>
        </ElementInspector>,
      );

      const target = screen.getByText('Fallback');
      const containerEl = container.firstElementChild as HTMLElement;

      vi.spyOn(containerEl, 'getBoundingClientRect').mockReturnValue({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
        top: 0,
        left: 0,
        right: 800,
        bottom: 600,
        toJSON: vi.fn(),
      });

      act(() => {
        fireEvent.click(target);
      });

      // Should use the first class as fallback since there's no -- variant
      expect(onSelect).toHaveBeenCalledWith(`.${TEST_CN_PREFIX}Base`);
    });
  });

  describe('component labels', () => {
    it('labels the nearest element carrying the label attribute on hover', () => {
      render(
        <ElementInspector enabled labelAttribute="data-preview-component">
          <div data-preview-component="<UserList />">
            <span>Aiko Tanaka</span>
          </div>
        </ElementInspector>,
      );

      fireEvent.mouseOver(screen.getByText('Aiko Tanaka'));

      expect(screen.getByText('<UserList />')).toBeTruthy();
    });

    it('shows nothing over content without a label', () => {
      render(
        <ElementInspector enabled labelAttribute="data-preview-component">
          <span>Plain</span>
        </ElementInspector>,
      );

      fireEvent.mouseOver(screen.getByText('Plain'));

      expect(screen.queryByText(/</)).toBeNull();
    });

    it('leaves clicks to the content', () => {
      const onClick = vi.fn();
      const onSelect = vi.fn();
      render(
        <ElementInspector enabled labelAttribute="data-preview-component" onSelectSelector={onSelect}>
          <button type="button" className={`${TEST_CN_PREFIX}Base`} onClick={onClick}>
            Add
          </button>
        </ElementInspector>,
      );

      fireEvent.click(screen.getByText('Add'));

      expect(onClick).toHaveBeenCalled();
      expect(onSelect).not.toHaveBeenCalled();
    });
  });
});
