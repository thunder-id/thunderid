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

import {Box, Typography, TextField, Stack, InputAdornment, Tooltip} from '@wso2/oxygen-ui';
import {
  Search,
  Smile,
  User,
  PawPrint,
  UtensilsCrossed,
  Plane,
  Trophy,
  Lightbulb,
  Hash,
  Flag,
} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, useMemo, useRef, useEffect, memo, type ComponentType, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import EMOJI_DATA from './emojis.json';

/**
 * Represents a single emoji icon with associated searchable keywords.
 */
export interface EmojiIcon {
  char: string;
  keywords: string;
}

/**
 * Represents a category of emoji icons.
 */
export interface EmojiCategory {
  label: string;
  emojis: EmojiIcon[];
}

const EMOJI_CATEGORIES: EmojiCategory[] = EMOJI_DATA as EmojiCategory[];

/**
 * Probes each emoji once via an offscreen canvas. Emojis that are unsupported
 * by the OS font render as a monochromatic "NO GLYPH" box — supported ones
 * produce at least one coloured pixel. Result is cached for the session.
 */
let supportedCategories: EmojiCategory[] | null = null;

function getSupportedCategories(): EmojiCategory[] {
  if (supportedCategories) return supportedCategories;
  if (typeof document === 'undefined') return EMOJI_CATEGORIES;

  const canvas = document.createElement('canvas');
  canvas.width = 20;
  canvas.height = 20;
  const ctx = canvas.getContext('2d');
  if (!ctx) return EMOJI_CATEGORIES;

  const hasGlyph = (char: string): boolean => {
    ctx.clearRect(0, 0, 20, 20);
    ctx.font = '14px serif';
    ctx.fillText(char, 0, 16);
    const {data} = ctx.getImageData(0, 0, 20, 20);
    for (let i = 0; i < data.length; i += 4) {
      const [r, g, b, a] = [data[i], data[i + 1], data[i + 2], data[i + 3]];
      if (a > 0 && (Math.abs(r - g) > 10 || Math.abs(g - b) > 10 || Math.abs(r - b) > 10)) {
        return true;
      }
    }
    return false;
  };

  supportedCategories = EMOJI_CATEGORIES.map(
    (cat): EmojiCategory => ({...cat, emojis: cat.emojis.filter((e) => hasGlyph(e.char))}),
  ).filter((cat): boolean => cat.emojis.length > 0);

  return supportedCategories;
}

const CATEGORY_ICON_MAP = new Map<string, ComponentType<{size?: number}>>([
  ['Smileys & Emotion', Smile],
  ['People & Body', User],
  ['Animals & Nature', PawPrint],
  ['Food & Drink', UtensilsCrossed],
  ['Travel & Places', Plane],
  ['Activities', Trophy],
  ['Objects', Lightbulb],
  ['Symbols', Hash],
  ['Flags', Flag],
] as [string, ComponentType<{size?: number}>][]);

const CATEGORY_I18N_KEYS: Record<string, string | undefined> = {
  'Smileys & Emotion': 'emoji_picker.categories.smileys_emotion',
  'People & Body': 'emoji_picker.categories.people_body',
  'Animals & Nature': 'emoji_picker.categories.animals_nature',
  'Food & Drink': 'emoji_picker.categories.food_drink',
  'Travel & Places': 'emoji_picker.categories.travel_places',
  Activities: 'emoji_picker.categories.activities',
  Objects: 'emoji_picker.categories.objects',
  Symbols: 'emoji_picker.categories.symbols',
  Flags: 'emoji_picker.categories.flags',
};

/**
 * Props for the {@link EmojiPicker} component.
 *
 * @public
 */
export interface EmojiPickerProps {
  /**
   * The currently highlighted emoji character (no `emoji:` prefix).
   */
  value?: string;

  /**
   * Fired when the user clicks an emoji tile.
   *
   * @param char - The raw emoji character.
   */
  onChange: (char: string) => void;
}

/**
 * A pure emoji-grid panel with a category filter bar and search.
 * Contains no dialog chrome — embed this inside a dialog or any other container.
 *
 * - Category tabs scroll the grid to that section and highlight as you scroll.
 * - Typing in the search field shows filtered results across all categories.
 * - Clicking an emoji tile fires {@link EmojiPickerProps.onChange} immediately.
 *
 * @public
 */
function EmojiPicker({value = '', onChange}: EmojiPickerProps): JSX.Element {
  const {t} = useTranslation('elements');
  const [search, setSearch] = useState<string>('');
  const allCategories: EmojiCategory[] = getSupportedCategories();
  const [activeCategory, setActiveCategory] = useState<string>(allCategories[0]?.label ?? '');

  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const sectionRefs = useRef<Map<string, HTMLDivElement>>(new Map());
  const isScrollingProgrammatically = useRef<boolean>(false);
  const pendingScrollLabel = useRef<string | null>(null);
  // Ref-copy of isSearching so handleCategoryClick stays stable (no dep on search)
  const isSearchingRef = useRef<boolean>(false);

  const isSearching: boolean = search.trim().length > 0;

  useEffect((): void => {
    isSearchingRef.current = isSearching;
  }, [isSearching]);

  const searchResults = useMemo((): EmojiCategory[] => {
    const query: string = search.trim().toLowerCase();
    if (!query) return [];
    return allCategories
      .map(
        (cat): EmojiCategory => ({
          ...cat,
          emojis: cat.emojis.filter((e) => e.keywords.toLowerCase().includes(query) || e.char === query),
        }),
      )
      .filter((cat): boolean => cat.emojis.length > 0);
  }, [search, allCategories]);

  const displayedSections: EmojiCategory[] = isSearching ? searchResults : allCategories;

  useEffect((): (() => void) | void => {
    if (isSearching) return;
    const container = scrollContainerRef.current;
    if (!container) return;

    const observer = new IntersectionObserver(
      (entries) => {
        if (isScrollingProgrammatically.current) return;
        const intersecting = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (intersecting.length > 0) {
          const label = (intersecting[0].target as HTMLElement).dataset['categoryLabel'];
          if (label) setActiveCategory(label);
        }
      },
      {root: container, rootMargin: '0px 0px -70% 0px', threshold: 0},
    );

    sectionRefs.current.forEach((el) => observer.observe(el));
    return (): void => observer.disconnect();
  }, [isSearching]);

  const scrollToLabel = useCallback((label: string): void => {
    isScrollingProgrammatically.current = true;
    const el = sectionRefs.current.get(label);
    if (el && scrollContainerRef.current && typeof scrollContainerRef.current.scrollTo === 'function') {
      scrollContainerRef.current.scrollTo({top: el.offsetTop, behavior: 'smooth'});
    }
    setTimeout((): void => {
      isScrollingProgrammatically.current = false;
    }, 600);
  }, []);

  const handleCategoryClick = useCallback(
    (label: string): void => {
      setActiveCategory(label);
      if (isSearchingRef.current) {
        // Section refs don't exist yet — defer scroll until search is cleared and sections re-mount
        pendingScrollLabel.current = label;
        setSearch('');
      } else {
        scrollToLabel(label);
      }
    },
    [scrollToLabel],
  );

  // Execute the deferred scroll after search clears and sections are re-mounted
  useEffect((): void => {
    if (!isSearching && pendingScrollLabel.current) {
      const label = pendingScrollLabel.current;
      pendingScrollLabel.current = null;
      scrollToLabel(label);
    }
  }, [isSearching, scrollToLabel]);

  const setSectionRef = useCallback((label: string, el: HTMLDivElement | null): void => {
    if (el) sectionRefs.current.set(label, el);
    else sectionRefs.current.delete(label);
  }, []);

  return (
    <Stack sx={{minWidth: 0}}>
      <Stack spacing={1.5} sx={{p: 1.5}}>
        {/* ── Search ── */}
        <TextField
          fullWidth
          size="small"
          aria-label={t('emoji_picker.search.label', 'Search emojis')}
          placeholder={t('emoji_picker.search.placeholder', 'Search emojis...')}
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            },
            htmlInput: {
              'aria-label': t('emoji_picker.search.label', 'Search emojis'),
            },
          }}
        />

        {/* ── Category filter bar ── */}
        <Box
          sx={{
            display: 'flex',
            overflowX: 'auto',
            pb: 1,
            '&::-webkit-scrollbar': {display: 'none'},
            scrollbarWidth: 'none',
          }}
        >
          {allCategories
            .filter((cat) => CATEGORY_ICON_MAP.has(cat.label))
            .map((cat) => {
              const {label} = cat;
              const Icon = CATEGORY_ICON_MAP.get(label)!;
              const isActive: boolean = !isSearching && activeCategory === label;
              return (
                <Tooltip key={label} title={t(CATEGORY_I18N_KEYS[label] ?? label, label)} placement="top">
                  <Box
                    component="button"
                    type="button"
                    onClick={() => handleCategoryClick(label)}
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      flexShrink: 0,
                      width: 40,
                      height: 44,
                      border: 'none',
                      borderBottom: '2px solid',
                      borderColor: isActive ? 'primary.main' : 'transparent',
                      background: 'none',
                      cursor: 'pointer',
                      color: isActive ? 'primary.main' : 'text.secondary',
                      transition: 'color 0.15s, border-color 0.15s',
                      '&:hover': {color: 'primary.main', bgcolor: 'action.hover'},
                    }}
                  >
                    <Icon size={20} />
                  </Box>
                </Tooltip>
              );
            })}
        </Box>

        {/* ── Scrollable emoji grid ── */}
        <Box ref={scrollContainerRef} sx={{height: 260, overflowY: 'auto', pr: 0.5}}>
          {displayedSections.length > 0 ? (
            displayedSections.map((section) => (
              <Box key={section.label} sx={{mb: 1.5}}>
                <Typography
                  ref={(el) => setSectionRef(section.label, el as HTMLDivElement | null)}
                  data-category-label={section.label}
                  variant="caption"
                  color="text.secondary"
                  sx={{fontWeight: 600, letterSpacing: '0.05em', textTransform: 'uppercase', mb: 0.5, display: 'block'}}
                >
                  {t(CATEGORY_I18N_KEYS[section.label] ?? section.label, section.label)}
                </Typography>
                <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 0.5}}>
                  {section.emojis.map((emoji) => (
                    <Box
                      key={emoji.char}
                      onClick={() => onChange(emoji.char)}
                      title={emoji.keywords}
                      sx={{
                        width: 36,
                        height: 36,
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        fontSize: '1.375rem',
                        cursor: 'pointer',
                        borderRadius: 1,
                        border: '2px solid',
                        borderColor: value === emoji.char ? 'primary.main' : 'transparent',
                        bgcolor: value === emoji.char ? 'primary.light' : 'transparent',
                        transition: 'all 0.1s',
                        '&:hover': {bgcolor: 'action.hover', borderColor: 'primary.light'},
                      }}
                    >
                      {emoji.char}
                    </Box>
                  ))}
                </Box>
              </Box>
            ))
          ) : (
            <Typography variant="body2" color="text.secondary" sx={{textAlign: 'center', py: 3}}>
              {t('emoji_picker.empty_state.message', 'No emojis found for "{{search}}"', {search})}
            </Typography>
          )}
        </Box>
      </Stack>
    </Stack>
  );
}

export default memo(EmojiPicker);
