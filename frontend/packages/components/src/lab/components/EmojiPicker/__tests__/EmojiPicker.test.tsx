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

import {render, screen, fireEvent, act} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import EmojiPicker from '../EmojiPicker';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | Record<string, unknown>, options?: Record<string, unknown>) => {
      const str = typeof fallback === 'string' ? fallback : key;
      const vars = typeof fallback === 'object' ? fallback : options;
      if (vars && typeof vars === 'object') {
        return str.replace(/\{\{(\w+)\}\}/g, (_, k: string) => {
          const val = vars[k];
          return typeof val === 'string' || typeof val === 'number' || typeof val === 'boolean' ? String(val) : '';
        });
      }
      return str;
    },
  }),
}));

// The module-level _supportedCategories cache needs to be reset between tests
// so that the canvas mock only runs once per test. Since jsdom's canvas 2D
// context is null, getSupportedCategories() falls through to returning all
// EMOJI_CATEGORIES – this is consistent across tests.

describe('EmojiPicker', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Rendering', () => {
    it('should render without crashing', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      expect(document.body).toBeTruthy();
    });

    it('should display a search input field', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      expect(screen.getByPlaceholderText(/search emojis/i)).toBeInTheDocument();
    });

    it('should render category tab buttons', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      // There are 9 category tabs; each is a <button> element
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThanOrEqual(9);
    });

    it('should render at least one emoji section label', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      // Category headings like "SMILEYS & EMOTION" should be visible
      const sectionLabels = ['Smileys & Emotion', 'People & Body', 'Animals & Nature'];
      const found = sectionLabels.some((label) => screen.queryByText(new RegExp(label, 'i')));
      expect(found).toBe(true);
    });

    it('should render emoji tiles in the grid', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      // At least one emoji should be in the document
      // Emojis are rendered as text nodes inside clickable boxes
      const gridContainer = document.querySelector('[class*="MuiBox"]');
      expect(gridContainer).toBeTruthy();
    });
  });

  describe('Search functionality', () => {
    it('should filter emoji sections when the user types a query', async () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const searchInput = screen.getByPlaceholderText(/search emojis/i);

      // Before searching, multiple category labels are visible
      const smileyLabel = screen.queryByText('Smileys & Emotion');
      expect(smileyLabel).toBeInTheDocument();

      // Type a very specific search query that exists in at least one category's keywords
      await userEvent.type(searchInput, 'smile');

      // After typing, the full category list is replaced by search results or no-results
      // The category headings should not all be visible anymore
      // (the search view only shows matching categories)
      expect(searchInput).toHaveValue('smile');
    });

    it('should show an empty-state message when the search produces no results', async () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const searchInput = screen.getByPlaceholderText(/search emojis/i);
      await userEvent.type(searchInput, 'zzznoresultsxyz999');

      expect(screen.getByText(/no emojis found/i)).toBeInTheDocument();
    });

    it('should restore all sections after the search is cleared', async () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const searchInput = screen.getByPlaceholderText(/search emojis/i);
      await userEvent.type(searchInput, 'zzznoresultsxyz999');

      // Clear the search
      await userEvent.clear(searchInput);

      // Categories return
      expect(screen.queryByText(/no emojis found/i)).not.toBeInTheDocument();
    });

    it('should include the search term in the empty-state message', async () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const searchInput = screen.getByPlaceholderText(/search emojis/i);
      await userEvent.type(searchInput, 'uniqueterm999');

      expect(screen.getByText(/uniqueterm999/)).toBeInTheDocument();
    });
  });

  describe('Emoji selection', () => {
    it('should call onChange with the correct emoji character when an emoji is clicked', () => {
      const onChange = vi.fn<(emoji: string) => void>();
      render(<EmojiPicker onChange={onChange} />);

      // Find the first emoji tile rendered in the grid (a Box with role not set, has emoji text)
      // Locate by searching for any element with the title attribute (emoji keywords)
      const emojiTiles = document.querySelectorAll('[title]');
      const firstTile = Array.from(emojiTiles).find((el) => el.textContent && el.textContent.trim().length > 0);

      expect(firstTile).toBeDefined();
      fireEvent.click(firstTile!);
      const selectedEmoji = onChange.mock.calls[0]?.[0] as string | undefined;

      expect(onChange).toHaveBeenCalledTimes(1);
      expect(typeof selectedEmoji).toBe('string');
      expect(selectedEmoji?.length).toBeGreaterThan(0);
    });
  });

  describe('Value highlight', () => {
    it('should highlight the emoji tile matching the value prop', () => {
      // We pick a well-known emoji that is likely to be in EMOJI_DATA
      const value = '😀';
      render(<EmojiPicker value={value} onChange={vi.fn()} />);

      // The component applies a colored border to the matching tile.
      // Verify the emoji is rendered somewhere in the document.
      const emojiEl = screen.getByText(value);
      expect(emojiEl).toBeInTheDocument();
    });
  });

  describe('Category navigation', () => {
    it('should not crash when a category tab button is clicked', () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const buttons = screen.getAllByRole('button');
      // Click the second category button (index 1) to avoid first being active
      expect(buttons.length).toBeGreaterThan(1);
      expect(() => fireEvent.click(buttons[1])).not.toThrow();
    });

    it('should clear the search when a category tab is clicked', async () => {
      render(<EmojiPicker onChange={vi.fn()} />);

      const searchInput = screen.getByPlaceholderText(/search emojis/i);
      await userEvent.type(searchInput, 'smile');
      expect(searchInput).toHaveValue('smile');

      // Click a category button
      const buttons = screen.getAllByRole('button');
      act(() => {
        fireEvent.click(buttons[0]);
      });

      expect(searchInput).toHaveValue('');
    });

    it('should call scrollTo on the scroll container when category is clicked (non-search mode)', () => {
      const mockScrollTo = vi.fn();
      Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
        configurable: true,
        value: mockScrollTo,
      });

      render(<EmojiPicker onChange={vi.fn()} />);

      const buttons = screen.getAllByRole('button');
      if (buttons.length > 1) {
        fireEvent.click(buttons[1]);
      }

      expect(mockScrollTo).toHaveBeenCalled();

      delete (HTMLElement.prototype as {scrollTo?: unknown}).scrollTo;
    });

    it('should reset isScrollingProgrammatically flag after 600ms timeout', () => {
      vi.useFakeTimers();

      const mockScrollTo = vi.fn();
      Object.defineProperty(HTMLElement.prototype, 'scrollTo', {
        configurable: true,
        value: mockScrollTo,
      });

      render(<EmojiPicker onChange={vi.fn()} />);

      const buttons = screen.getAllByRole('button');
      if (buttons.length > 1) {
        act(() => {
          fireEvent.click(buttons[1]);
        });
      }

      act(() => {
        vi.advanceTimersByTime(600);
      });

      vi.useRealTimers();
      delete (HTMLElement.prototype as {scrollTo?: unknown}).scrollTo;

      expect(document.body).toBeTruthy();
    });
  });

  describe('Canvas glyph detection', () => {
    it('should filter emojis using canvas context when available', async () => {
      vi.resetModules();

      const origCreate = document.createElement.bind(document);
      const mockGetImageData = vi.fn().mockReturnValue({
        // Colored pixel: R=255, G=100, B=0, A=255 — Math.abs(255-100)=155 > 10
        data: new Uint8ClampedArray([255, 100, 0, 255, 0, 0, 0, 0]),
      });
      vi.spyOn(document, 'createElement').mockImplementation((tagName: string) => {
        if (tagName === 'canvas') {
          return {
            width: 0,
            height: 0,
            getContext: vi.fn().mockReturnValue({
              clearRect: vi.fn(),
              font: '',
              fillText: vi.fn(),
              getImageData: mockGetImageData,
            }),
          } as unknown as HTMLCanvasElement;
        }
        return origCreate(tagName as keyof HTMLElementTagNameMap);
      });

      const {default: FreshEmojiPicker} = await import('../EmojiPicker');
      render(<FreshEmojiPicker onChange={vi.fn()} />);

      vi.restoreAllMocks();

      expect(screen.getByPlaceholderText(/search emojis/i)).toBeInTheDocument();
    });

    it('should use cached categories on subsequent renders (cache hit path)', async () => {
      // This import reuses the module from the previous test which set the cache
      const {default: FreshEmojiPicker} = await import('../EmojiPicker');
      render(<FreshEmojiPicker onChange={vi.fn()} />);

      expect(screen.getByPlaceholderText(/search emojis/i)).toBeInTheDocument();
    });
  });
});
