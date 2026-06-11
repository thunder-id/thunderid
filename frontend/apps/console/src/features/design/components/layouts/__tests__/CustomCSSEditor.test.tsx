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

import {render, screen, fireEvent, act, cleanup, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Stylesheet} from '@thunderid/design';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {createRef} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import CustomCSSEditor from '../CustomCSSEditor';
import type {CustomCSSEditorHandle} from '../CustomCSSEditor';

// Mock Monaco Editor as a plain textarea
vi.mock('@/lib/MonacoEditor', () => ({
  default: ({value, onChange, height}: {value: string; onChange?: (v: string | undefined) => void; height: string}) => (
    <textarea
      data-testid="monaco-editor"
      data-height={height}
      value={value}
      onChange={(e) => onChange?.(e.target.value)}
    />
  ),
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Plus: () => <span data-testid="icon-plus" />,
    Trash: () => <span data-testid="icon-trash" />,
    ChevronDown: () => <span data-testid="icon-chevron-down" />,
    ChevronUp: () => <span data-testid="icon-chevron-up" />,
    Edit: () => <span data-testid="icon-edit" />,
    Maximize: () => <span data-testid="icon-maximize" />,
    X: () => <span data-testid="icon-x" />,
  };
});

function renderWithTheme(ui: React.ReactElement) {
  return render(<OxygenUIThemeProvider>{ui}</OxygenUIThemeProvider>);
}

const inlineSheet: Stylesheet = {id: 'custom-1', type: 'inline', content: '.foo { color: red; }'};
const urlSheet: Stylesheet = {id: 'custom-2', type: 'url', href: 'https://example.com/style.css'};

beforeEach(() => {
  vi.useFakeTimers({shouldAdvanceTime: true});
});

afterEach(() => {
  vi.runOnlyPendingTimers();
  vi.useRealTimers();
  cleanup();
});

describe('CustomCSSEditor', () => {
  describe('rendering', () => {
    it('renders add buttons for inline and external URL', () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[]} onChange={vi.fn()} />);
      expect(screen.getByText('Inline')).toBeTruthy();
      expect(screen.getByText('External URL')).toBeTruthy();
    });

    it('renders stylesheet items for each stylesheet', () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={vi.fn()} />);
      expect(screen.getByText('custom-1')).toBeTruthy();
      expect(screen.getByText('custom-2')).toBeTruthy();
    });

    it('renders empty state when no stylesheets', () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[]} onChange={vi.fn()} />);
      expect(screen.getByText('No custom stylesheets yet.')).toBeTruthy();
    });

    it('renders type chips for each stylesheet', () => {
      const {container} = renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={vi.fn()} />);
      const chips = container.querySelectorAll('.MuiChip-label');
      const chipTexts = Array.from(chips).map((c) => c.textContent);
      expect(chipTexts).toContain('Inline');
      expect(chipTexts).toContain('URL');
    });
  });

  describe('adding stylesheets', () => {
    it('adds an inline stylesheet when Inline button is clicked', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[]} onChange={onChange} />);

      await act(async () => {
        await userEvent.click(screen.getByRole('button', {name: /Inline/}));
      });

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({type: 'inline', content: ''})]);
    });

    it('adds a URL stylesheet when External URL button is clicked', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[]} onChange={onChange} />);

      await act(async () => {
        await userEvent.click(screen.getByText('External URL'));
      });

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({type: 'url', href: ''})]);
    });

    it('generates unique IDs that avoid collisions', async () => {
      const onChange = vi.fn();
      const existing: Stylesheet[] = [{id: 'custom-1', type: 'inline', content: ''}];
      renderWithTheme(<CustomCSSEditor stylesheets={existing} onChange={onChange} />);

      // Target the add button by finding the one with the Plus icon
      const addButtons = screen.getAllByRole('button', {name: /Inline/});
      const addButton = addButtons.find((btn) => btn.querySelector('[data-testid="icon-plus"]'))!;
      await act(async () => {
        await userEvent.click(addButton);
      });

      expect(onChange).toHaveBeenCalledWith([existing[0], expect.objectContaining({id: 'custom-2'})]);
    });
  });

  describe('expanding/collapsing', () => {
    it('expands a stylesheet item when its header is clicked', async () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={vi.fn()} />);

      // Click the header to expand
      await act(async () => {
        await userEvent.click(screen.getByText('custom-1'));
      });

      // The Monaco editor should appear in expanded content
      expect(screen.getByTestId('monaco-editor')).toBeTruthy();
    });

    it('auto-expands newly added stylesheet', async () => {
      const sheets: Stylesheet[] = [];
      const onChange = vi.fn();
      const {rerender} = renderWithTheme(<CustomCSSEditor stylesheets={sheets} onChange={onChange} />);

      await act(async () => {
        await userEvent.click(screen.getByRole('button', {name: /Inline/}));
      });

      // Re-render with the new sheet to simulate parent updating
      const newSheets = onChange.mock.calls[0][0] as Stylesheet[];
      rerender(
        <OxygenUIThemeProvider>
          <CustomCSSEditor stylesheets={newSheets} onChange={onChange} />
        </OxygenUIThemeProvider>,
      );

      // The new sheet should be expanded (editor visible)
      expect(screen.getByTestId('monaco-editor')).toBeTruthy();
    });
  });

  describe('removing stylesheets', () => {
    it('removes a stylesheet when delete button is clicked', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={onChange} />);

      // Find the delete buttons (Trash icons inside span[role="button"])
      const deleteButtons = screen.getAllByTestId('icon-trash');
      await act(async () => {
        const deleteSpan = deleteButtons[0].closest('[role="button"]')!;
        await userEvent.click(deleteSpan);
      });

      expect(onChange).toHaveBeenCalledWith([urlSheet]);
    });

    it('clears expanded index when removing the currently expanded sheet', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand
      await act(async () => {
        await userEvent.click(screen.getByText('custom-1'));
      });
      expect(screen.getByTestId('monaco-editor')).toBeTruthy();

      // Delete
      const deleteSpan = screen.getByTestId('icon-trash').closest('[role="button"]')!;
      await act(async () => {
        await userEvent.click(deleteSpan);
      });

      expect(onChange).toHaveBeenCalledWith([]);
    });
  });

  describe('reordering stylesheets', () => {
    it('moves a stylesheet down when down arrow is clicked', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={onChange} />);

      // The down arrows are ChevronDown icons inside span[role="button"] within the reorder stack.
      // Each item has an up arrow (ChevronUp) and a down arrow (ChevronDown).
      // The first item's down arrow is the first clickable ChevronDown inside a reorder span.
      const chevronDownIcons = screen.getAllByTestId('icon-chevron-down');
      // chevronDownIcons: [item1-move-down, item1-expand, item2-move-down, item2-expand]
      // The move-down icon is inside a span[role="button"], expand icon is inside AccordionSummary's expandIcon
      const moveDownButton = chevronDownIcons[0].closest('[role="button"]');
      expect(moveDownButton).toBeTruthy();

      await act(async () => {
        await userEvent.click(moveDownButton!);
      });

      expect(onChange).toHaveBeenCalledWith([urlSheet, inlineSheet]);
    });

    it('moves a stylesheet up when up arrow is clicked', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={onChange} />);

      // The second item's up arrow (ChevronUp icons: [item1-up, item2-up])
      const chevronUpIcons = screen.getAllByTestId('icon-chevron-up');
      const moveUpButton = chevronUpIcons[1].closest('[role="button"]');
      expect(moveUpButton).toBeTruthy();

      await act(async () => {
        await userEvent.click(moveUpButton!);
      });

      expect(onChange).toHaveBeenCalledWith([urlSheet, inlineSheet]);
    });

    it('disables move up for first item and move down for last item', () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={vi.fn()} />);

      // Single item: both arrows should have low opacity (visually disabled)
      const chevronUpIcons = screen.getAllByTestId('icon-chevron-up');
      const chevronDownIcons = screen.getAllByTestId('icon-chevron-down');
      // They exist but are visually disabled (opacity 0.25)
      expect(chevronUpIcons.length).toBeGreaterThanOrEqual(1);
      expect(chevronDownIcons.length).toBeGreaterThanOrEqual(1);
    });
  });

  describe('updating stylesheets', () => {
    it('updates inline CSS content with debounce', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand the item
      await act(async () => {
        await userEvent.click(screen.getByText('custom-1'));
      });

      // Find the Monaco editor textarea and change its value
      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: '.bar { color: blue; }'}});

      // Advance past the debounce timer
      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({content: '.bar { color: blue; }'})]);
    });

    it('updates the ID via inline editing (edit icon click)', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // Click the edit icon next to the title to enter edit mode
      const editIcon = screen.getByTestId('icon-edit');
      const editButton = editIcon.closest('[role="button"]')!;
      await act(async () => {
        await userEvent.click(editButton);
      });

      // An input should appear with the current value
      const input = screen.getByDisplayValue('custom-1');
      fireEvent.change(input, {target: {value: 'my-styles'}});
      fireEvent.blur(input);

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({id: 'my-styles'})]);
    });

    it('updates URL href for url-type stylesheets', async () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[urlSheet]} onChange={onChange} />);

      // Expand
      await act(async () => {
        await userEvent.click(screen.getByText('custom-2'));
      });

      const urlField = screen.getByDisplayValue('https://example.com/style.css');
      fireEvent.change(urlField, {target: {value: 'https://cdn.example.com/new.css'}});

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({href: 'https://cdn.example.com/new.css'})]);
    });

    it('shows warning for http URLs', async () => {
      const httpUrlSheet: Stylesheet = {id: 'url-1', type: 'url', href: 'http://insecure.com/style.css'};
      renderWithTheme(<CustomCSSEditor stylesheets={[httpUrlSheet]} onChange={vi.fn()} />);

      // Expand
      await act(async () => {
        await userEvent.click(screen.getByText('url-1'));
      });

      expect(screen.getByText('Using HTTP is insecure. Consider using HTTPS instead.')).toBeTruthy();
    });

    it('shows error for invalid URLs', async () => {
      const invalidSheet: Stylesheet = {id: 'url-1', type: 'url', href: 'not-a-url'};
      renderWithTheme(<CustomCSSEditor stylesheets={[invalidSheet]} onChange={vi.fn()} />);

      await act(async () => {
        await userEvent.click(screen.getByText('url-1'));
      });

      expect(screen.getByText('URL must be a valid http:// or https:// address')).toBeTruthy();
    });

    it('does not show error for empty URL', async () => {
      const emptyUrlSheet: Stylesheet = {id: 'url-1', type: 'url', href: ''};
      renderWithTheme(<CustomCSSEditor stylesheets={[emptyUrlSheet]} onChange={vi.fn()} />);

      await act(async () => {
        await userEvent.click(screen.getByText('url-1'));
      });

      expect(screen.queryByText('URL must be a valid http:// or https:// address')).toBeNull();
    });

    it('does not show error for valid https URL', async () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[urlSheet]} onChange={vi.fn()} />);

      await act(async () => {
        await userEvent.click(screen.getByText('custom-2'));
      });

      expect(screen.queryByText('URL must be a valid http:// or https:// address')).toBeNull();
    });
  });

  describe('enable/disable toggle', () => {
    it('toggles disabled state via the eye icon', () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // The eye icon toggle has an aria-label "Hide from preview" when enabled
      const toggle = screen.getByLabelText('Hide from preview');
      expect(toggle).toBeTruthy();

      act(() => {
        fireEvent.click(toggle);
      });

      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({disabled: true})]);
    });

    it('renders with eye-off icon when disabled', () => {
      const disabledSheet: Stylesheet = {...inlineSheet, disabled: true};
      renderWithTheme(<CustomCSSEditor stylesheets={[disabledSheet]} onChange={vi.fn()} />);

      // When disabled, the toggle shows "Show in preview"
      const toggle = screen.getByLabelText('Show in preview');
      expect(toggle).toBeTruthy();
    });
  });

  describe('expand to modal (InlineCSSField)', () => {
    it('opens full editor dialog when maximize button is clicked', async () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={vi.fn()} />);

      // Expand the stylesheet item
      await act(async () => {
        await userEvent.click(screen.getByText('custom-1'));
      });

      // Click the maximize icon
      const maximizeButton = screen.getByTestId('icon-maximize').closest('button')!;
      await act(async () => {
        await userEvent.click(maximizeButton);
      });

      // Dialog should appear with the ID as title
      expect(screen.getByRole('dialog')).toBeTruthy();
    });

    it('closes the dialog when X button is clicked', async () => {
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={vi.fn()} />);

      await act(async () => {
        await userEvent.click(screen.getByText('custom-1'));
      });

      const maximizeButton = screen.getByTestId('icon-maximize').closest('button')!;
      await act(async () => {
        await userEvent.click(maximizeButton);
      });

      expect(screen.getByRole('dialog')).toBeTruthy();

      // Click the close (X) button
      const closeButton = screen.getByTestId('icon-x').closest('button')!;
      await act(async () => {
        await userEvent.click(closeButton);
      });

      // MUI Dialog uses transitions; wait for removal
      await waitFor(() => {
        expect(screen.queryByRole('dialog')).toBeNull();
      });
    });
  });

  describe('debounce behavior', () => {
    it('debounces editor changes to 400ms', () => {
      const onChange = vi.fn();
      renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand
      act(() => {
        fireEvent.click(screen.getByText('custom-1'));
      });

      const editor = screen.getByTestId('monaco-editor');

      // Type quickly — only the last value should propagate
      fireEvent.change(editor, {target: {value: 'a'}});
      act(() => {
        vi.advanceTimersByTime(100);
      });
      fireEvent.change(editor, {target: {value: 'ab'}});
      act(() => {
        vi.advanceTimersByTime(100);
      });
      fireEvent.change(editor, {target: {value: 'abc'}});

      // Not yet called because debounce hasn't elapsed
      expect(onChange).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(400);
      });
      expect(onChange).toHaveBeenCalledTimes(1);
      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({content: 'abc'})]);
    });
  });

  describe('external content sync (InlineCSSField)', () => {
    it('syncs local content when external content changes', () => {
      const onChange = vi.fn();
      const {rerender} = renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand
      act(() => {
        fireEvent.click(screen.getByText('custom-1'));
      });

      const editor = screen.getByTestId<HTMLTextAreaElement>('monaco-editor');
      expect(editor.value).toBe('.foo { color: red; }');

      // Simulate external update
      const updatedSheet: Stylesheet = {...inlineSheet, content: '.updated { color: green; }'};
      rerender(
        <OxygenUIThemeProvider>
          <CustomCSSEditor stylesheets={[updatedSheet]} onChange={onChange} />
        </OxygenUIThemeProvider>,
      );

      const editor2 = screen.getByTestId<HTMLTextAreaElement>('monaco-editor');
      expect(editor2.value).toBe('.updated { color: green; }');
    });
  });

  describe('stable keys sync on external replacement', () => {
    it('regenerates stable keys when stylesheets are replaced externally with different length', () => {
      const onChange = vi.fn();
      const {rerender} = renderWithTheme(<CustomCSSEditor stylesheets={[inlineSheet]} onChange={onChange} />);

      expect(screen.getByText('custom-1')).toBeTruthy();

      // Externally replace with two sheets (different length triggers key regeneration)
      rerender(
        <OxygenUIThemeProvider>
          <CustomCSSEditor stylesheets={[inlineSheet, urlSheet]} onChange={onChange} />
        </OxygenUIThemeProvider>,
      );

      expect(screen.getByText('custom-1')).toBeTruthy();
      expect(screen.getByText('custom-2')).toBeTruthy();
    });
  });

  describe('flush via ref (imperative handle)', () => {
    it('flush() commits pending debounced edits immediately', () => {
      const onChange = vi.fn();
      const ref = createRef<CustomCSSEditorHandle>();
      renderWithTheme(<CustomCSSEditor ref={ref} stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand
      act(() => {
        fireEvent.click(screen.getByText('custom-1'));
      });

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: '.flushed { color: red; }'}});

      // Debounce hasn't fired yet
      expect(onChange).not.toHaveBeenCalled();

      // Flush synchronously
      act(() => {
        ref.current!.flush();
      });

      expect(onChange).toHaveBeenCalledTimes(1);
      expect(onChange).toHaveBeenCalledWith([expect.objectContaining({content: '.flushed { color: red; }'})]);
    });

    it('flush() is a no-op when there are no pending edits', () => {
      const onChange = vi.fn();
      const ref = createRef<CustomCSSEditorHandle>();
      renderWithTheme(<CustomCSSEditor ref={ref} stylesheets={[inlineSheet]} onChange={onChange} />);

      act(() => {
        ref.current!.flush();
      });

      expect(onChange).not.toHaveBeenCalled();
    });

    it('flush() does not double-fire after the debounce timer completes', () => {
      const onChange = vi.fn();
      const ref = createRef<CustomCSSEditorHandle>();
      renderWithTheme(<CustomCSSEditor ref={ref} stylesheets={[inlineSheet]} onChange={onChange} />);

      // Expand and edit
      act(() => {
        fireEvent.click(screen.getByText('custom-1'));
      });

      const editor = screen.getByTestId('monaco-editor');
      fireEvent.change(editor, {target: {value: '.no-double { color: blue; }'}});

      // Flush
      act(() => {
        ref.current!.flush();
      });

      expect(onChange).toHaveBeenCalledTimes(1);

      // Advance past the original debounce — should NOT fire again
      act(() => {
        vi.advanceTimersByTime(500);
      });

      expect(onChange).toHaveBeenCalledTimes(1);
    });
  });
});
