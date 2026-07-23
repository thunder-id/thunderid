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

import {render, screen, fireEvent, within} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import LogoPicker from '../LogoPicker';

/**
 * A flyout's option tiles live in a sibling Box right after its header Stack (see
 * `FlyoutContent` in LogoPicker.tsx) — traversing from the flyout's label text is a more
 * reliable way to scope queries to just those options than guessing at emoji/glyph text.
 */
function getFlyoutOptions(flyoutLabel: string): HTMLElement {
  const label = screen.getByText(flyoutLabel);
  return label.parentElement!.nextElementSibling as HTMLElement;
}

describe('LogoPicker', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  describe('custom image URL field', () => {
    it('should not call onChange while the URL is incomplete', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https:/exam'}});
      vi.advanceTimersByTime(1000);

      expect(handleChange).not.toHaveBeenCalled();
    });

    it('should not call onChange for text that is not a URL', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'example'}});
      fireEvent.blur(urlField);

      expect(handleChange).not.toHaveBeenCalled();
    });

    it('should call onChange after typing stops on a valid URL', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https://example.com/logo.png'}});

      expect(handleChange).not.toHaveBeenCalled();
      vi.advanceTimersByTime(500);

      expect(handleChange).toHaveBeenCalledWith('https://example.com/logo.png');
    });

    it('should call onChange immediately on blur with a valid URL', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https://example.com/logo.png'}});
      fireEvent.blur(urlField);

      expect(handleChange).toHaveBeenCalledWith('https://example.com/logo.png');
    });

    it('should not call onChange on blur when the URL is still incomplete', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      const urlField = screen.getByPlaceholderText(/paste an image url/i);
      fireEvent.change(urlField, {target: {value: 'https://'}});
      fireEvent.blur(urlField);

      expect(handleChange).not.toHaveBeenCalled();
    });
  });

  describe('variant flyouts', () => {
    it('should mark the just-picked emoji as selected and keep the flyout open', () => {
      const handleChange = vi.fn();
      const {rerender} = render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      fireEvent.click(screen.getByText('Emoji').previousElementSibling!);
      const options = getFlyoutOptions('Choose an emoji');
      const [firstOption] = within(options).getAllByRole('button');
      fireEvent.click(firstOption);

      expect(handleChange).toHaveBeenCalledTimes(1);
      const pickedValue = handleChange.mock.calls[0][0] as string;

      // The flyout stays open after picking, so re-rendering with the committed value...
      rerender(<LogoPicker value={pickedValue} onChange={handleChange} />);
      expect(screen.getByText('Choose an emoji')).toBeInTheDocument();

      // ...and the picked emoji is marked selected among the still-visible options.
      const refreshedOptions = getFlyoutOptions('Choose an emoji');
      const pickedOption = within(refreshedOptions).getByText(pickedValue.slice('emoji:'.length)).closest('button')!;
      expect(pickedOption).toHaveAttribute('aria-pressed', 'true');
    });

    it('should keep the avatar swatch flyout open after picking a background', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      fireEvent.click(screen.getByText('Avatar').previousElementSibling!);
      const options = getFlyoutOptions('Pick a background');
      const [firstSwatch] = within(options).getAllByRole('button');
      fireEvent.click(firstSwatch);

      expect(handleChange).toHaveBeenCalledTimes(1);
      expect(screen.getByText('Pick a background')).toBeInTheDocument();
    });

    it('should keep the animal flyout open after picking an icon', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      fireEvent.click(screen.getByText('Animal').previousElementSibling!);
      const options = getFlyoutOptions('Choose an animal');
      const [firstIcon] = within(options).getAllByRole('button');
      fireEvent.click(firstIcon);

      expect(handleChange).toHaveBeenCalledTimes(1);
      expect(screen.getByText('Choose an animal')).toBeInTheDocument();
    });

    it('should keep the entity flyout open after picking an icon', () => {
      const handleChange = vi.fn();
      render(<LogoPicker value="emoji:🎉" onChange={handleChange} />);

      fireEvent.click(screen.getByText('Entity').previousElementSibling!);
      const options = getFlyoutOptions('Choose an icon');
      const [firstIcon] = within(options).getAllByRole('button');
      fireEvent.click(firstIcon);

      expect(handleChange).toHaveBeenCalledTimes(1);
      expect(screen.getByText('Choose an icon')).toBeInTheDocument();
    });
  });
});
