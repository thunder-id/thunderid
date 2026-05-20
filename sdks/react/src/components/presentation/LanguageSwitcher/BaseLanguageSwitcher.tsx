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

import {cx} from '@emotion/css';
import {
  autoUpdate,
  flip,
  FloatingFocusManager,
  FloatingPortal,
  offset,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useRole,
} from '@floating-ui/react';
import {FC, ReactElement, ReactNode, useEffect, useState} from 'react';
import useStyles from './BaseLanguageSwitcher.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import Check from '../../primitives/Icons/Check';
import ChevronDown from '../../primitives/Icons/ChevronDown';

/**
 * A resolved language option with display name and emoji flag.
 */
export interface LanguageOption {
  /** BCP 47 language tag (e.g. "en", "fr", "en-US") */
  code: string;
  /** Human-readable display name resolved via Intl.DisplayNames */
  displayName: string;
  /** Flag emoji or globe emoji for the language */
  emoji: string;
}

/**
 * Render props exposed to consumers when using the render-prop pattern.
 */
export interface LanguageSwitcherRenderProps {
  /** The currently active language code */
  currentLanguage: string;
  /** Whether a language switch is in progress */
  isLoading: boolean;
  /** Resolved language options */
  languages: LanguageOption[];
  /** Call this to switch to a different language */
  onLanguageChange: (language: string) => void;
}

export interface BaseLanguageSwitcherProps {
  /**
   * Render-props callback. When provided, the default dropdown UI is replaced with
   * whatever JSX the callback returns.
   *
   * @example
   * ```tsx
   * <BaseLanguageSwitcher {...props}>
   *   {({languages, currentLanguage, onLanguageChange}) => (
   *     <select value={currentLanguage} onChange={e => onLanguageChange(e.target.value)}>
   *       {languages.map(l => <option key={l.code} value={l.code}>{l.emoji} {l.displayName}</option>)}
   *     </select>
   *   )}
   * </BaseLanguageSwitcher>
   * ```
   */
  children?: (props: LanguageSwitcherRenderProps) => ReactNode;
  /** Additional CSS class applied to the root element (default UI only) */
  className?: string;
  /** The currently active language code */
  currentLanguage: string;
  /** Whether a language switch is in progress */
  isLoading?: boolean;
  /** Resolved language options to display */
  languages: LanguageOption[];
  /** Called when the user selects a language */
  onLanguageChange: (language: string) => void;
}

/**
 * Pure-UI language switcher dropdown.
 * Accepts resolved `LanguageOption[]` (code + displayName + emoji) and delegates
 * language switching to the `onLanguageChange` callback.
 *
 * Supports render props for full UI customisation.
 */
const BaseLanguageSwitcher: FC<BaseLanguageSwitcherProps> = ({
  children,
  className,
  currentLanguage,
  isLoading = false,
  languages,
  onLanguageChange,
}: BaseLanguageSwitcherProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme);
  const [isOpen, setIsOpen] = useState(false);
  const hasMultipleLanguages: boolean = languages.length > 1;

  useEffect(() => {
    if (!hasMultipleLanguages && isOpen) {
      setIsOpen(false);
    }
  }, [hasMultipleLanguages, isOpen]);

  const {refs, floatingStyles, context} = useFloating({
    middleware: [offset(4), flip(), shift()],
    onOpenChange: setIsOpen,
    open: isOpen,
    whileElementsMounted: autoUpdate,
  });

  const click: ReturnType<typeof useClick> = useClick(context, {enabled: hasMultipleLanguages});
  const dismiss: ReturnType<typeof useDismiss> = useDismiss(context, {enabled: hasMultipleLanguages});
  const role: ReturnType<typeof useRole> = useRole(context, {enabled: hasMultipleLanguages, role: 'listbox'});
  const {getReferenceProps, getFloatingProps} = useInteractions([click, dismiss, role]);

  const currentOption: LanguageOption | undefined = languages.find((l: LanguageOption) => l.code === currentLanguage);

  if (children) {
    return (
      <>
        {children({
          currentLanguage,
          isLoading,
          languages,
          onLanguageChange,
        })}
      </>
    );
  }

  return (
    <div className={cx(styles['root'], className)}>
      <button
        ref={refs.setReference}
        type="button"
        disabled={isLoading}
        aria-label="Switch language"
        {...getReferenceProps()}
        className={styles['trigger']}
      >
        {currentOption && <span className={styles['triggerEmoji']}>{currentOption.emoji}</span>}
        <span className={styles['triggerLabel']}>{currentOption?.displayName ?? currentLanguage}</span>
        {hasMultipleLanguages && <ChevronDown />}
      </button>

      {isOpen && hasMultipleLanguages && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false}>
            <div
              ref={refs.setFloating}
              style={floatingStyles}
              {...getFloatingProps()}
              className={styles['content']}
              role="listbox"
              aria-label="Select language"
            >
              {languages.map((lang: LanguageOption) => (
                <button
                  key={lang.code}
                  type="button"
                  role="option"
                  aria-selected={lang.code === currentLanguage}
                  className={cx(styles['option'], lang.code === currentLanguage && styles['optionActive'])}
                  onClick={() => {
                    onLanguageChange(lang.code);
                    setIsOpen(false);
                  }}
                >
                  <span className={styles['optionEmoji']}>{lang.emoji}</span>
                  <span className={styles['optionLabel']}>{lang.displayName}</span>
                  {lang.code === currentLanguage && (
                    <span className={styles['checkIcon']}>
                      <Check />
                    </span>
                  )}
                </button>
              ))}
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </div>
  );
};

export default BaseLanguageSwitcher;
