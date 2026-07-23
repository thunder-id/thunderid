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

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {AppWindow} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {describe, it, expect, vi} from 'vitest';
import ResourceAvatar from '../ResourceAvatar';

/**
 * Mirrors how a real caller wires `ResourceAvatar` — `onSelect` feeds back into the
 * `value` prop — so the dialog's Cancel-to-revert logic (which diffs against the
 * live `value` prop) has something to actually diff against.
 */
function ControlledResourceAvatar({
  initialValue,
  onSelectSpy,
  onSave = undefined,
}: {
  initialValue: string;
  onSelectSpy: (value: string) => void;
  onSave?: () => void | Promise<void>;
}): JSX.Element {
  const [value, setValue] = useState<string>(initialValue);
  return (
    <ResourceAvatar
      editable
      value={value}
      onSave={onSave}
      onSelect={(newValue) => {
        onSelectSpy(newValue);
        setValue(newValue);
      }}
    />
  );
}

describe('ResourceAvatar', () => {
  describe('Read-only mode (no onSelect)', () => {
    it('should render the fallback icon when no value is provided', () => {
      render(<ResourceAvatar fallback={<AppWindow data-testid="fallback-icon" />} />);

      expect(screen.getByTestId('fallback-icon')).toBeInTheDocument();
    });

    it('should render the emoji character when value is emoji:-prefixed', () => {
      render(<ResourceAvatar value="emoji:🎉" />);

      expect(screen.getByText('🎉')).toBeInTheDocument();
    });

    it('should render the raw emoji character when value has no prefix', () => {
      render(<ResourceAvatar value="🐼" />);

      expect(screen.getByText('🐼')).toBeInTheDocument();
    });

    it('should pass the URL as an img src when value is a URL', () => {
      render(<ResourceAvatar value="https://example.com/logo.png" />);

      const img = screen.getByRole('img');
      expect(img).toHaveAttribute('src', 'https://example.com/logo.png');
    });

    it('should not render an edit button when editable and onSelect are not provided', () => {
      render(<ResourceAvatar value="emoji:🎉" />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('should call onClick when avatar is clicked in read-only mode', () => {
      const handleClick = vi.fn();
      render(<ResourceAvatar value="emoji:🎉" onClick={handleClick} />);

      const avatar = screen.getByText('🎉').closest('[class*="Avatar"]') ?? screen.getByText('🎉').parentElement!;
      fireEvent.click(avatar);

      expect(handleClick).toHaveBeenCalledTimes(1);
    });

    it('should render with emoji value', () => {
      render(<ResourceAvatar value="emoji:🎉" />);

      expect(screen.getByText('🎉')).toBeInTheDocument();
    });
  });

  describe('Edit mode (onSelect provided)', () => {
    it('should render an edit (pencil) button when onSelect and editable are provided', () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} />);

      expect(screen.getByRole('button', {name: 'Change logo'})).toBeInTheDocument();
    });

    it('should have the default aria-label "Change logo" on the edit button', () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} />);

      expect(screen.getByRole('button', {name: 'Change logo'})).toBeInTheDocument();
    });

    it('should accept a custom editAriaLabel', () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} editAriaLabel="Update icon" />);

      expect(screen.getByRole('button', {name: 'Update icon'})).toBeInTheDocument();
    });

    it('should open the logo picker dialog when the edit button is clicked', () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should open the logo picker dialog when the avatar itself is clicked', () => {
      render(<ResourceAvatar value="emoji:🎉" onSelect={vi.fn()} />);

      fireEvent.click(screen.getByText('🎉'));

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should close the dialog when the close button is clicked', async () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      expect(screen.getByRole('dialog')).toBeInTheDocument();

      fireEvent.click(screen.getByRole('button', {name: /close/i}));
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('should call onSelect as the user picks a logo, without closing the dialog', () => {
      const handleSelect = vi.fn();
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={handleSelect} />);

      // Open the dialog
      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));

      // Enter a custom image URL; the picker commits it once the field loses focus
      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https://example.com/logo.png'}});
      fireEvent.blur(urlField);

      expect(handleSelect).toHaveBeenCalledWith('https://example.com/logo.png');
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should show fallback icon inside avatar when no value provided in edit mode', () => {
      render(<ResourceAvatar editable fallback={<AppWindow data-testid="fallback-icon" />} onSelect={vi.fn()} />);

      expect(screen.getByTestId('fallback-icon')).toBeInTheDocument();
    });

    it('should pre-select the string fallback spec in the picker when no value is set', () => {
      render(
        <ResourceAvatar
          editable
          fallback="avatar:shape=rounded,variant=anonymous_entity,content=pavilion,colors=0"
          onSelect={vi.fn()}
        />,
      );

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));

      // Before the fix, the picker opened on an empty emoji instead of the fallback avatar,
      // so its preview tile rendered as blank text rather than an image.
      const dialog = screen.getByRole('dialog');
      const previewImages = dialog.querySelectorAll('img');
      expect(previewImages.length).toBeGreaterThan(0);
    });

    it('should revert to the original value and close the dialog when Cancel is clicked', async () => {
      const handleSelect = vi.fn();
      render(<ControlledResourceAvatar initialValue="emoji:🎉" onSelectSpy={handleSelect} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https://example.com/logo.png'}});
      fireEvent.blur(urlField);
      expect(handleSelect).toHaveBeenCalledWith('https://example.com/logo.png');

      fireEvent.click(screen.getByRole('button', {name: /cancel/i}));

      expect(handleSelect).toHaveBeenLastCalledWith('emoji:🎉');
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('should not call onSelect on Cancel when nothing changed', () => {
      const handleSelect = vi.fn();
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={handleSelect} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      fireEvent.click(screen.getByRole('button', {name: /cancel/i}));

      expect(handleSelect).not.toHaveBeenCalled();
    });

    it('should close the dialog on Save without calling onSave when none is provided', async () => {
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      fireEvent.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('should call onSave and close the dialog when Save is clicked', async () => {
      const handleSave = vi.fn().mockResolvedValue(undefined);
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} onSave={handleSave} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      fireEvent.click(screen.getByRole('button', {name: /^save$/i}));

      expect(handleSave).toHaveBeenCalledTimes(1);
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });

    it('should keep the dialog open and not throw when onSave rejects', async () => {
      const handleSave = vi.fn().mockRejectedValue(new Error('save failed'));
      render(<ResourceAvatar editable value="emoji:🎉" onSelect={vi.fn()} onSave={handleSave} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      fireEvent.click(screen.getByRole('button', {name: /^save$/i}));

      await waitFor(() => expect(screen.getByRole('button', {name: /^save$/i})).not.toBeDisabled());
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should ignore Cancel and disable the close button while a save is in flight', async () => {
      const handleSelect = vi.fn();
      let resolveSave: () => void = () => undefined;
      const handleSave = vi.fn(
        () =>
          new Promise<void>((resolve) => {
            resolveSave = resolve;
          }),
      );
      render(<ControlledResourceAvatar initialValue="emoji:🎉" onSelectSpy={handleSelect} onSave={handleSave} />);

      fireEvent.click(screen.getByRole('button', {name: 'Change logo'}));
      fireEvent.click(screen.getByRole('button', {name: /^save$/i}));

      const closeButton = screen.getByRole('button', {name: /close/i});
      expect(closeButton).toBeDisabled();

      fireEvent.click(closeButton);
      expect(screen.getByRole('dialog')).toBeInTheDocument();
      expect(handleSelect).not.toHaveBeenCalled();

      resolveSave();
      await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });
  });
});
