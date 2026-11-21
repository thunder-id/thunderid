// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Menu, MenuItem} from '@wso2/oxygen-ui';
import {ChevronDown, Globe} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type MouseEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {storeAppLanguage} from '../hooks/useAppLanguage';

/** A language's own name for itself, such as `American English` for `en-US`. */
const nativeName = (language: string): string => {
  try {
    return new Intl.DisplayNames([language], {type: 'language'}).of(language) ?? language;
  } catch {
    return language;
  }
};

/**
 * The top bar's language menu: the languages MyAccount offers, each in its own name. The pick is
 * kept for the next visit.
 */
export default function LanguageMenu({offered}: {offered: string[]}): JSX.Element {
  const {i18n, t} = useTranslation();
  const [anchor, setAnchor] = useState<HTMLElement | null>(null);

  const choose = (language: string): void => {
    setAnchor(null);
    storeAppLanguage(language);
    void i18n.changeLanguage(language);
  };

  return (
    <>
      <Button
        size="small"
        variant="outlined"
        color="inherit"
        startIcon={<Globe size={16} />}
        endIcon={<ChevronDown size={14} />}
        aria-label={t('account:language.label', 'Language')}
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        onClick={(event: MouseEvent<HTMLElement>) => setAnchor(event.currentTarget)}
        sx={{borderRadius: 5, borderColor: 'divider', color: 'text.primary', minWidth: 0}}
      >
        {i18n.language.split('-')[0].toUpperCase()}
      </Button>
      <Menu anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)}>
        {offered.map((language: string) => (
          <MenuItem
            key={language}
            lang={language}
            selected={language === i18n.language}
            onClick={() => choose(language)}
          >
            {nativeName(language)}
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
