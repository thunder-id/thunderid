// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {TextField, type TextFieldProps} from '@wso2/oxygen-ui';
import {useEffect, useRef, useState, type ChangeEvent, type KeyboardEvent, type ReactElement} from 'react';

/**
 * Props interface of {@link DraftTextField}
 */
export type DraftTextFieldPropsInterface = Omit<TextFieldProps, 'value' | 'onChange' | 'onBlur' | 'onKeyDown'> & {
  /**
   * The committed property value.
   */
  value: string;
  /**
   * Commits the edited value. Called on blur and on Enter, never per keystroke.
   */
  onCommit: (value: string) => void;
  /**
   * Coerces a raw draft into the value to commit (clamping, rounding, defaults).
   * Returning `null` rejects the edit and restores {@link value}.
   */
  normalize?: (raw: string) => string | null;
};

/**
 * Text field for executor properties that keeps the in-progress edit in local state
 * and commits it once the user is done.
 *
 * Committing a property is expensive — it clones the step's component tree, writes to
 * the React Flow store and re-renders the property panel — and these fields have no
 * canvas preview that needs to follow the keystrokes. Holding the draft locally keeps
 * typing at native speed, and committing on blur/Enter avoids the mid-typing commits
 * that would otherwise reset the caret.
 *
 * @param props - Props injected to the component.
 * @returns The DraftTextField component.
 */
function DraftTextField({value, onCommit, normalize = undefined, ...rest}: DraftTextFieldPropsInterface): ReactElement {
  const [draft, setDraft] = useState<string>(value);
  // The last `value` prop this field has seen, so an external change is distinguishable
  // from a parent that simply has not echoed a commit back yet.
  const [syncedValue, setSyncedValue] = useState<string>(value);
  // What this field has already handed upwards. Pressing Enter and then blurring both
  // reach `commit`, so this is what makes the second one a no-op.
  const [committed, setCommitted] = useState<string>(value);

  // Re-sync when the value changes outside this field — a different resource is selected,
  // or an undo/redo or plugin rewrote the property. Adjusting during render rather than in
  // an effect avoids rendering the stale value first.
  if (value !== syncedValue) {
    setSyncedValue(value);
    setDraft(value);
    setCommitted(value);
  }

  // Resolves what the draft should store, or null when the edit is rejected outright.
  const resolve = (): string | null => (normalize ? normalize(draft) : draft);

  const commit = (): void => {
    const next: string | null = resolve();

    if (next === null) {
      setDraft(committed);
      return;
    }

    // Normalization can rewrite the draft without changing the stored value (typing
    // `99` into a field that clamps at 20 when 20 is already stored), so the field is
    // reset to the normalized text whether or not a commit follows.
    setDraft(next);

    if (next !== committed) {
      setCommitted(next);
      onCommit(next);
    }
  };

  // Removing a focused input does not reliably fire blur, so a field that unmounts while
  // being edited — the panel closing, or a mode change hiding the field — would otherwise
  // drop the edit. The cleanup only commits; it never touches state after unmount.
  const pendingCommitRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    pendingCommitRef.current = (): void => {
      // Only an edit in progress may be flushed. Normalization can turn an untouched draft
      // into a different value (an empty field falling back to its minimum), so an unedited
      // field is left alone rather than storing a value the user never entered.
      if (draft === committed) {
        return;
      }

      const next: string | null = resolve();

      if (next !== null && next !== committed) {
        onCommit(next);
      }
    };
  });

  useEffect(
    () => () => {
      pendingCommitRef.current?.();
    },
    [],
  );

  return (
    <TextField
      {...rest}
      value={draft}
      onChange={(e: ChangeEvent<HTMLInputElement>) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
        // Multiline fields take Enter as a newline.
        if (e.key === 'Enter' && !rest.multiline) {
          commit();
        }
      }}
    />
  );
}

export default DraftTextField;
