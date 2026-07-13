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

import {Box, Button, ListItemIcon, ListItemText, ListSubheader, Menu, MenuItem, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, Check, ChevronDown, Heart, MessageCircle, MousePointer2} from '@wso2/oxygen-ui-icons-react';
import {extractPageMarkdownFromDocument} from 'docusaurus-plugin-copy-page-button/src/htmlToMarkdown';
import {JSX, MouseEvent, ReactNode, useState} from 'react';
import {getActionButtonSx, getSplitButtonStyles} from './actionButtonStyles';
import ClaudeLogo from './icons/ClaudeLogo';
import MarkdownIcon from './icons/MarkdownIcon';
import OpenAIIcon from './icons/OpenAIIcon';
import useIsDarkMode from '../hooks/useIsDarkMode';

// The plugin ships no type declarations; pin down the signature once here.
const extractMarkdown = extractPageMarkdownFromDocument as (documentLike: Document, pageUrl: string) => string;

type AssistantGroup = 'LLM' | 'IDE' | 'BUILDER';

interface AssistantItem {
  id: string;
  group: AssistantGroup;
  title: string;
  icon: ReactNode;
  buildUrl: (prompt: string) => string;
}

// Only services with a confirmed, documented URL scheme for pre-filling a prompt are
// listed here. Grok, Perplexity, Meta AI, Zed, and Bolt were investigated but have no
// verified scheme and were deliberately left out rather than guessed.
const ASSISTANT_ITEMS: AssistantItem[] = [
  {
    id: 'chatgpt',
    group: 'LLM',
    title: 'Open in ChatGPT',
    icon: <OpenAIIcon size={16} />,
    buildUrl: (prompt) => `https://chatgpt.com/?${new URLSearchParams({hints: 'search', prompt}).toString()}`,
  },
  {
    id: 'claude',
    group: 'LLM',
    title: 'Open in Claude',
    icon: <ClaudeLogo size={16} />,
    buildUrl: (prompt) => `https://claude.ai/new?${new URLSearchParams({q: prompt}).toString()}`,
  },
  {
    id: 't3',
    group: 'LLM',
    title: 'Open in T3 Chat',
    icon: <MessageCircle size={16} />,
    buildUrl: (prompt) => `https://t3.chat/new?${new URLSearchParams({q: prompt}).toString()}`,
  },
  {
    id: 'cursor',
    group: 'IDE',
    title: 'Open in Cursor',
    icon: <MousePointer2 size={16} />,
    buildUrl: (prompt) => `https://cursor.com/link/prompt?${new URLSearchParams({text: prompt}).toString()}`,
  },
  {
    id: 'v0',
    group: 'BUILDER',
    title: 'Open in v0',
    icon: (
      <Box component="span" sx={{fontFamily: "'JetBrains Mono', monospace", fontSize: '11px', fontWeight: 600, lineHeight: 1}}>
        v0
      </Box>
    ),
    buildUrl: (prompt) => `https://v0.app?${new URLSearchParams({q: prompt}).toString()}`,
  },
  {
    id: 'lovable',
    group: 'BUILDER',
    title: 'Open in Lovable',
    icon: <Heart size={16} />,
    buildUrl: (prompt) => `https://lovable.dev/?autosubmit=true#prompt=${encodeURIComponent(prompt)}`,
  },
];

const GROUP_ORDER: AssistantGroup[] = ['LLM', 'IDE', 'BUILDER'];
const DEFAULT_ITEM_ID = 'chatgpt';

// T3 is hidden for now but left in ASSISTANT_ITEMS so it can be re-enabled via `enabledIds`.
const DEFAULT_ENABLED_IDS: string[] = ASSISTANT_ITEMS.filter((item) => item.id !== 't3').map((item) => item.id);

function buildPrompt(pageUrl: string, group: AssistantGroup): string {
  if (group === 'IDE') {
    return `Read this documentation page and use it to implement the integration in my project: ${pageUrl}`;
  }
  if (group === 'BUILDER') {
    return `Read this documentation page and use it as a reference while building: ${pageUrl}`;
  }
  return `Please read and explain this page: ${pageUrl}\n\nProvide a clear summary and help me understand the key points covered.`;
}

interface ListRowProps {
  icon: ReactNode;
  label: string;
  onClick: () => void;
  isLight: boolean;
  showArrow?: boolean;
}

function ListRow({icon, label, onClick, isLight, showArrow = true}: ListRowProps): JSX.Element {
  return (
    <Box
      component="button"
      type="button"
      onClick={onClick}
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1.5,
        width: '100%',
        p: 1.25,
        border: 'none',
        borderRadius: '10px',
        bgcolor: 'transparent',
        cursor: 'pointer',
        textAlign: 'left',
        font: 'inherit',
        transition: 'background-color 0.15s ease',
        '&:hover': {bgcolor: isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.05)'},
      }}
    >
      <Box sx={{display: 'inline-flex', flexShrink: 0, color: 'text.primary'}}>{icon}</Box>
      <Typography sx={{flex: 1, fontSize: '14.5px', color: 'text.primary'}}>{label}</Typography>
      {showArrow && (
        <Box component="span" sx={{display: 'inline-flex', opacity: 0.5, color: 'text.secondary'}}>
          <ArrowUpRight size={15} />
        </Box>
      )}
    </Box>
  );
}

export type AIPageActionsVariant = 'buttons' | 'list';

export default function AIPageActions({
  variant = 'buttons',
  enabledIds = DEFAULT_ENABLED_IDS,
}: {
  variant?: AIPageActionsVariant;
  enabledIds?: string[];
}): JSX.Element | null {
  const isLight = !useIsDarkMode();
  const [copied, setCopied] = useState(false);
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);
  const items = ASSISTANT_ITEMS.filter((item) => enabledIds.includes(item.id));
  const defaultItem = items.find((item) => item.id === DEFAULT_ITEM_ID) ?? items[0];

  const handleCopyMarkdown = async () => {
    try {
      const markdown = extractMarkdown(document, window.location.href);
      if (!markdown) return;
      await navigator.clipboard.writeText(markdown);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard API unavailable or denied — silently ignore.
    }
  };

  const openItem = (item: AssistantItem) => {
    window.open(item.buildUrl(buildPrompt(window.location.href, item.group)), '_blank', 'noopener,noreferrer');
    setAnchorEl(null);
  };

  if (variant === 'list') {
    return (
      <Box>
        <Typography sx={{fontSize: '17px', fontWeight: 700, color: 'text.primary', mb: 1}}>Explore with AI</Typography>
        <Box sx={{display: 'flex', flexDirection: 'column'}}>
          {items.map((item) => (
            <ListRow key={item.id} icon={item.icon} label={item.title} onClick={() => openItem(item)} isLight={isLight} />
          ))}
          <Box sx={{height: '1px', my: 0.5, bgcolor: isLight ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.06)'}} />
          <ListRow
            icon={copied ? <Check size={16} /> : <MarkdownIcon size={16} />}
            label={copied ? 'Copied' : 'Copy as markdown'}
            onClick={() => void handleCopyMarkdown()}
            isLight={isLight}
            showArrow={false}
          />
        </Box>
      </Box>
    );
  }

  if (!defaultItem) {
    return null;
  }

  const actionSx = getActionButtonSx(isLight);
  const splitSx = getSplitButtonStyles(isLight);

  return (
    <Box sx={{display: 'flex', flexDirection: 'column', gap: 1}}>
      <Button
        fullWidth
        size="small"
        variant="outlined"
        onClick={() => void handleCopyMarkdown()}
        startIcon={copied ? <Check size={14} /> : <MarkdownIcon size={16} />}
        sx={actionSx}
      >
        {copied ? 'Copied' : 'Copy as markdown'}
      </Button>

      <Box sx={splitSx.container}>
        <Box component="button" type="button" onClick={() => openItem(defaultItem)} sx={splitSx.main}>
          {defaultItem.icon}
          {defaultItem.title}
        </Box>
        <Box sx={splitSx.divider} />
        <Box
          component="button"
          type="button"
          onClick={(event: MouseEvent<HTMLButtonElement>) => setAnchorEl(event.currentTarget)}
          aria-haspopup="true"
          aria-label="More AI assistants"
          sx={splitSx.chevron}
        >
          <ChevronDown size={13} />
        </Box>
      </Box>
      <Menu anchorEl={anchorEl} open={Boolean(anchorEl)} onClose={() => setAnchorEl(null)}>
        {GROUP_ORDER.flatMap((group) => {
          const groupItems = items.filter((item) => item.group === group);
          if (groupItems.length === 0) return [];
          return [
            <ListSubheader key={`${group}-label`} sx={{fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', lineHeight: '28px'}}>
              {group}
            </ListSubheader>,
            ...groupItems.map((item) => (
              <MenuItem key={item.id} onClick={() => openItem(item)}>
                <ListItemIcon>{item.icon}</ListItemIcon>
                <ListItemText>{item.title}</ListItemText>
                <Box component="span" sx={{display: 'inline-flex', opacity: 0.5, ml: 2}}>
                  <ArrowUpRight size={14} />
                </Box>
              </MenuItem>
            )),
          ];
        })}
      </Menu>
    </Box>
  );
}
