// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, renderHook, screen, act, fireEvent} from '@thunderid/test-utils';
import {useTranslation} from 'react-i18next';
import {describe, expect, it, vi, beforeAll, beforeEach, afterEach} from 'vitest';
import TranslationJsonEditor from '@/components/edit-translation/TranslationJsonEditor';

// Monaco Editor is not available in jsdom; replace it with a plain textarea
// that mirrors the same value/onChange contract.
vi.mock('@monaco-editor/react', () => ({
  default: ({value, onChange}: {value: string; onChange?: (v: string | undefined) => void}) => (
    <textarea data-testid="monaco-editor" value={value} onChange={(e) => onChange?.(e.target.value)} />
  ),
}));

const sampleValues = {'actions.save': 'Save', 'actions.cancel': 'Cancel'};
const sampleServerKeys = Object.keys(sampleValues);

// Helper: fire a change event on the editor and advance the 400ms debounce.
// userEvent.type() deadlocks under vi.useFakeTimers() because its internal
// per-keystroke delays also use setTimeout; fireEvent.change() is synchronous
// and avoids the issue entirely.
function changeEditor(editor: HTMLElement, value: string) {
  fireEvent.change(editor, {target: {value}});
  act(() => {
    vi.advanceTimersByTime(400);
  });
}

describe('TranslationJsonEditor', () => {
  let t: (key: string) => string;

  beforeAll(() => {
    ({t} = renderHook(() => useTranslation('translations')).result.current);
  });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('Rendering', () => {
    it('renders the Monaco editor with the initial JSON value', () => {
      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      const editor = screen.getByTestId('monaco-editor');
      const parsed = JSON.parse((editor as HTMLTextAreaElement).value) as Record<string, string>;

      expect(parsed).toEqual(sampleValues);
    });

    it('does not show the invalid-JSON warning on initial render', () => {
      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      expect(screen.queryByText(t('editor.jsonInvalid'))).not.toBeInTheDocument();
    });
  });

  describe('Valid JSON changes', () => {
    it('calls onChange with the parsed record after the debounce fires', () => {
      const onChange = vi.fn();

      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={onChange}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), JSON.stringify({'actions.save': 'Enregistrer'}));

      expect(onChange).toHaveBeenCalledWith({'actions.save': 'Enregistrer'});
    });

    it('does not show the invalid-JSON warning for valid JSON', () => {
      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), '{"key": "value"}');

      expect(screen.queryByText(t('editor.jsonInvalid'))).not.toBeInTheDocument();
    });
  });

  describe('Invalid JSON handling', () => {
    it('shows a warning alert when the editor contains invalid JSON', () => {
      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), '{not valid json');

      expect(screen.getByText(t('editor.jsonInvalid'))).toBeInTheDocument();
    });

    it('does not call onChange while JSON is invalid', () => {
      const onChange = vi.fn();

      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={onChange}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), '{invalid');

      expect(onChange).not.toHaveBeenCalled();
    });

    it('does not show the warning alert when the editor is empty', () => {
      render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), '');

      expect(screen.queryByText(t('editor.jsonInvalid'))).not.toBeInTheDocument();
    });
  });

  describe('External value updates', () => {
    it('syncs the editor when values prop changes to a new object reference', () => {
      const {rerender} = render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      const newValues = {'page.title': 'My Page'};
      rerender(
        <TranslationJsonEditor
          values={newValues}
          serverKeys={Object.keys(newValues)}
          isKeyCreationAllowedNamespace={false}
          colorMode="light"
          onChange={vi.fn()}
        />,
      );

      const editor = screen.getByTestId('monaco-editor');
      const parsed = JSON.parse((editor as HTMLTextAreaElement).value) as Record<string, string>;

      expect(parsed).toEqual(newValues);
    });

    it('does not reformat the editor text when the values prop echoes back its own onChange call', () => {
      const onChange = vi.fn();

      const {rerender} = render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace
          colorMode="light"
          onChange={onChange}
        />,
      );

      // A new key typed in the middle of the document, rather than appended at the end.
      const typed = '{\n  "actions.save": "Save",\n  "new.key": "New Value",\n  "actions.cancel": "Cancel"\n}';
      changeEditor(screen.getByTestId('monaco-editor'), typed);

      const expectedRecord = {'actions.save': 'Save', 'new.key': 'New Value', 'actions.cancel': 'Cancel'};
      expect(onChange).toHaveBeenCalledWith(expectedRecord);

      // The parent merges the change and passes a freshly-built object back down, as
      // `{...serverValues, ...localChanges}` would — a new reference, same shape.
      rerender(
        <TranslationJsonEditor
          values={{...expectedRecord}}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace
          colorMode="light"
          onChange={onChange}
        />,
      );

      // JSON.stringify would have moved "new.key" to the end (insertion order); the editor
      // must keep showing exactly what the user typed instead of snapping to that reformat.
      const editor = screen.getByTestId('monaco-editor');
      expect((editor as HTMLTextAreaElement).value).toBe(typed);
    });

    it('still syncs from an external change immediately after a self-triggered one', () => {
      const onChange = vi.fn();

      const {rerender} = render(
        <TranslationJsonEditor
          values={sampleValues}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace
          colorMode="light"
          onChange={onChange}
        />,
      );

      changeEditor(screen.getByTestId('monaco-editor'), '{"actions.save": "Enregistrer"}');
      expect(onChange).toHaveBeenCalledWith({'actions.save': 'Enregistrer'});

      // Self-triggered echo — consumes the skip-once flag.
      rerender(
        <TranslationJsonEditor
          values={{'actions.save': 'Enregistrer'}}
          serverKeys={sampleServerKeys}
          isKeyCreationAllowedNamespace
          colorMode="light"
          onChange={onChange}
        />,
      );

      // A genuinely external change (e.g. switching namespace) must still sync normally.
      const externalValues = {'page.title': 'My Page'};
      rerender(
        <TranslationJsonEditor
          values={externalValues}
          serverKeys={Object.keys(externalValues)}
          isKeyCreationAllowedNamespace
          colorMode="light"
          onChange={onChange}
        />,
      );

      const editor = screen.getByTestId('monaco-editor');
      const parsed = JSON.parse((editor as HTMLTextAreaElement).value) as Record<string, string>;
      expect(parsed).toEqual(externalValues);
    });
  });
});
