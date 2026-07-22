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

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ResourceLogoDialog from '../ResourceLogoDialog';

const defaultProps = {
  open: true,
  onClose: vi.fn(),
  onSelect: vi.fn(),
};

describe('ResourceLogoDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Visibility', () => {
    it('should render the dialog when open is true', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });

    it('should not render the dialog content when open is false', () => {
      render(<ResourceLogoDialog {...defaultProps} open={false} />);

      expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });
  });

  describe('Rendering', () => {
    it('should show a dialog title', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByText(/choose a logo/i)).toBeInTheDocument();
    });

    it('should show a Cancel button', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByRole('button', {name: /cancel/i})).toBeInTheDocument();
    });

    it('should show a Select button', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByRole('button', {name: /^select$/i})).toBeInTheDocument();
    });

    it('should show a URL input field', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i)).toBeInTheDocument();
    });

    it('should show the search emojis field from EmojiPicker', () => {
      render(<ResourceLogoDialog {...defaultProps} />);

      expect(screen.getByPlaceholderText(/search emojis/i)).toBeInTheDocument();
    });
  });

  describe('Initial state when dialog opens', () => {
    it('should disable the Select button when no value is pre-populated', () => {
      render(<ResourceLogoDialog {...defaultProps} value="" />);

      const selectButton = screen.getByRole('button', {name: /^select$/i});
      expect(selectButton).toBeDisabled();
    });

    it('should enable the Select button when an emoji value is pre-populated', () => {
      render(<ResourceLogoDialog {...defaultProps} value="emoji:🎉" />);

      const selectButton = screen.getByRole('button', {name: /^select$/i});
      expect(selectButton).not.toBeDisabled();
    });

    it('should enable the Select button when a URL value is pre-populated', () => {
      render(<ResourceLogoDialog {...defaultProps} value="https://example.com/logo.png" />);

      const selectButton = screen.getByRole('button', {name: /^select$/i});
      expect(selectButton).not.toBeDisabled();
    });

    it('should populate the URL input field when value is a URL', () => {
      render(<ResourceLogoDialog {...defaultProps} value="https://example.com/logo.png" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      expect(urlInput).toHaveValue('https://example.com/logo.png');
    });

    it('should clear the URL input when value is an emoji', () => {
      render(<ResourceLogoDialog {...defaultProps} value="emoji:🎉" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      expect(urlInput).toHaveValue('');
    });
  });

  describe('User interaction — Cancel', () => {
    it('should call onClose when the Cancel button is clicked', () => {
      const onClose = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onClose={onClose} />);

      fireEvent.click(screen.getByRole('button', {name: /cancel/i}));

      expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('should call onClose when the close icon button (X) is clicked', () => {
      const onClose = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onClose={onClose} />);

      const closeIconButton = screen.getByRole('button', {name: /close/i});
      fireEvent.click(closeIconButton);

      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('User interaction — URL selection', () => {
    it('should enable the Select button after entering a URL', () => {
      render(<ResourceLogoDialog {...defaultProps} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'https://example.com/icon.png'}});

      expect(screen.getByRole('button', {name: /^select$/i})).not.toBeDisabled();
    });

    it('should call onSelect with the raw URL when Select is clicked after entering a URL', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'https://example.com/icon.png'}});

      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));

      expect(onSelect).toHaveBeenCalledWith('https://example.com/icon.png');
    });

    it('should call onClose after calling onSelect', () => {
      const onClose = vi.fn();
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onClose={onClose} onSelect={onSelect} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'https://example.com/icon.png'}});

      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));

      expect(onClose).toHaveBeenCalledTimes(1);
    });
  });

  describe('User interaction — pre-populated emoji selection', () => {
    it('should call onSelect with emoji: prefix when Select is clicked with pre-populated emoji', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="emoji:🎉" />);

      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));

      expect(onSelect).toHaveBeenCalledWith('emoji:🎉');
    });
  });

  describe('User interaction — URL clears pending emoji', () => {
    it('should prefer URL over emoji when both could be set', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="emoji:🎉" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'https://example.com/icon.png'}});

      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));

      // URL takes priority over emoji in the selection logic
      expect(onSelect).toHaveBeenCalledWith('https://example.com/icon.png');
    });
  });

  describe('User interaction — emoji picker click', () => {
    it('should enable the Select button when an emoji tile is clicked in the picker', () => {
      render(<ResourceLogoDialog {...defaultProps} value="" />);

      // The EmojiPicker renders emoji tiles with a title attribute (emoji keywords)
      const emojiTiles = document.querySelectorAll('[title]');
      const firstTile = Array.from(emojiTiles).find((el) => el.textContent && el.textContent.trim().length > 0);

      expect(firstTile).toBeDefined();
      fireEvent.click(firstTile!);
      expect(screen.getByRole('button', {name: /^select$/i})).not.toBeDisabled();
    });

    it('should clear the URL input when an emoji is clicked after a URL is typed', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="" />);

      // Type a URL first
      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'https://example.com/icon.png'}});

      // Click an emoji tile to override the URL
      const emojiTiles = document.querySelectorAll('[title]');
      const firstTile = Array.from(emojiTiles).find((el) => el.textContent && el.textContent.trim().length > 0);

      expect(firstTile).toBeDefined();
      fireEvent.click(firstTile!);

      // URL input should be cleared
      expect(urlInput).toHaveValue('');
    });
  });

  describe('User interaction — URL validation', () => {
    it('should keep the Select button disabled when the entered URL is invalid', () => {
      render(<ResourceLogoDialog {...defaultProps} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'not a url'}});

      expect(screen.getByRole('button', {name: /^select$/i})).toBeDisabled();
    });

    it('should show an inline error message when the entered URL is invalid', () => {
      render(<ResourceLogoDialog {...defaultProps} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'javascript:alert(1)'}});

      expect(screen.getByText(/enter a valid image url/i)).toBeInTheDocument();
    });

    it('should not call onSelect when Select is clicked with an invalid URL', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: 'example.com/logo.png'}});
      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));

      expect(onSelect).not.toHaveBeenCalled();
    });

    it('should accept an absolute path as a valid logo URL', () => {
      const onSelect = vi.fn();
      render(<ResourceLogoDialog {...defaultProps} onSelect={onSelect} value="" />);

      const urlInput = screen.getByPlaceholderText(/https:\/\/example\.com\/logo\.png/i);
      fireEvent.change(urlInput, {target: {value: '/assets/logo.png'}});

      expect(screen.getByRole('button', {name: /^select$/i})).not.toBeDisabled();
      fireEvent.click(screen.getByRole('button', {name: /^select$/i}));
      expect(onSelect).toHaveBeenCalledWith('/assets/logo.png');
    });
  });

  describe('Initial state — plain emoji value without prefix', () => {
    it('should pre-populate emoji state when value is a raw emoji character without emoji: prefix', () => {
      render(<ResourceLogoDialog {...defaultProps} value="🎉" />);

      // The Select button should be enabled because pendingEmoji is set to '🎉'
      const selectButton = screen.getByRole('button', {name: /^select$/i});
      expect(selectButton).not.toBeDisabled();
    });
  });
});
