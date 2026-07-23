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

import {Box} from '@wso2/oxygen-ui';
import React, {useRef} from 'react';
import PythonLogo from './icons/PythonLogo';
import TypeScriptLogo from './icons/TypeScriptLogo';
import {useUrlSelection} from '@site/src/utils/useUrlSelection';

// The selected language shared by every <LangTabs> on the page, so switching the
// language on one block switches all of them. It lives in the URL query string
// (?lang=...) rather than a store: that keeps the choice shareable via the link
// and in sync across blocks, and a fresh visit with no query starts on the
// default language.
type Lang = 'python' | 'typescript';

const DEFAULT_LANG: Lang = 'python';

const LANGUAGES: {value: Lang; label: string; Icon: (props: {size?: number}) => React.ReactElement}[] = [
  {value: 'python', label: 'Python', Icon: PythonLogo},
  {value: 'typescript', label: 'TypeScript', Icon: TypeScriptLogo},
];

const LANG_VALUES = LANGUAGES.map(l => l.value) as readonly Lang[];

interface LangProps {
  value: Lang;
  children?: React.ReactNode;
}

// One language panel. It stays in the DOM and is hidden unless its language is
// the active one, matching Docusaurus tabs so both languages remain searchable
// rather than existing only after a click.
export function Lang({value, children = null}: LangProps): React.ReactElement {
  const [active] = useUrlSelection('lang', LANG_VALUES, DEFAULT_LANG);
  return (
    <Box role="tabpanel" hidden={value !== active}>
      {children}
    </Box>
  );
}

export function LangTabs({children = null}: {children?: React.ReactNode}): React.ReactElement {
  const [active, setLang] = useUrlSelection('lang', LANG_VALUES, DEFAULT_LANG);
  const tabRefs = useRef<(HTMLLIElement | null)[]>([]);

  // Roving tabindex: only the active tab is focusable; arrow keys move focus
  // across the tabs and select as they go, so keyboard users can switch too.
  function onKeyDown(e: React.KeyboardEvent<HTMLLIElement>, index: number): void {
    let next = index;
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') next = (index + 1) % LANGUAGES.length;
    else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') next = (index - 1 + LANGUAGES.length) % LANGUAGES.length;
    else if (e.key === 'Home') next = 0;
    else if (e.key === 'End') next = LANGUAGES.length - 1;
    else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      setLang(LANGUAGES[index].value);
      return;
    } else return;

    e.preventDefault();
    setLang(LANGUAGES[next].value);
    tabRefs.current[next]?.focus();
  }

  return (
    <Box className="tabs-container">
      <Box component="ul" role="tablist" aria-label="Language" className="tabs">
        {LANGUAGES.map(({value, label, Icon}, index) => {
          const isActive = active === value;
          return (
            <Box
              component="li"
              key={value}
              ref={(el: HTMLLIElement | null) => {
                tabRefs.current[index] = el;
              }}
              role="tab"
              tabIndex={isActive ? 0 : -1}
              aria-selected={isActive}
              className={`tabs__item${isActive ? ' tabs__item--active' : ''}`}
              onClick={() => setLang(value)}
              onKeyDown={(e: React.KeyboardEvent<HTMLLIElement>) => onKeyDown(e, index)}
            >
              <Box component="span" sx={{alignItems: 'center', display: 'inline-flex', gap: '0.45rem'}}>
                <Icon size={16} />
                {label}
              </Box>
            </Box>
          );
        })}
      </Box>
      <Box className="margin-top--md">{children}</Box>
    </Box>
  );
}
