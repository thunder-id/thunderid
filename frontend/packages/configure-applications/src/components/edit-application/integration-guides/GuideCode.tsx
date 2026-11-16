// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BunIcon, DenoIcon, NpmIcon, PnpmIcon, YarnIcon} from '@thunderid/components';
import {useCopyToClipboard} from '@thunderid/hooks';
import {alpha, Box, CodeBlock, IconButton, Stack, Tab, Tabs, Tooltip, Typography} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {fillQuickstartGuideCode} from '../../../utils/parseQuickstartGuide';

/** The logo of a package manager a code sample option is named after, by its name in lower case. */
const OPTION_ICONS: Record<string, JSX.Element> = {
  npm: <NpmIcon size={14} />,
  yarn: <YarnIcon size={14} />,
  pnpm: <PnpmIcon size={14} />,
  bun: <BunIcon size={14} />,
  deno: <DenoIcon size={14} />,
};

/** The languages the docs code samples are written in, mapped to the ones Oxygen UI's code block colors. */
const HIGHLIGHTED_LANGUAGES: Record<string, NonNullable<CodeBlockLanguage>> = {
  js: 'javascript',
  javascript: 'javascript',
  mjs: 'javascript',
  cjs: 'javascript',
  ts: 'typescript',
  typescript: 'typescript',
  jsx: 'jsx',
  tsx: 'tsx',
  css: 'css',
  bash: 'bash',
  sh: 'bash',
  shell: 'bash',
  json: 'json',
  html: 'html',
  vue: 'markup',
  xml: 'markup',
};

type CodeBlockLanguage = Parameters<typeof CodeBlock>[0]['language'];

/**
 * One variant of a code sample, such as the command for one package manager.
 *
 * @public
 */
export interface GuideCodeOption {
  /** The option's name, shown as its tab (e.g. "npm") */
  label?: string;
  /** The language of the code */
  language?: string;
  /** The code itself */
  code: string;
}

/**
 * Props for the {@link GuideCode} component.
 *
 * @public
 */
export interface GuideCodeProps {
  /** A caption for a single-option sample, such as its file name. Defaults to its language. */
  title?: string;
  /** The variants of the sample. More than one renders as tabs. */
  options: GuideCodeOption[];
  /** Placeholder text in the code, mapped to this application's value for it */
  codeValues: Record<string, string | undefined>;
  /** The option last picked, applied across every sample on the page */
  codeOption?: string;
  /** Picks an option */
  onCodeOptionChange?: (label: string) => void;
}

/**
 * A code sample with a copy button. Several options render as tabs, one per option. Values filled
 * in from the application are highlighted so they stand apart from the sample's own code.
 *
 * @public
 */
export default function GuideCode({
  title = undefined,
  options,
  codeValues,
  codeOption = undefined,
  onCodeOptionChange = undefined,
}: GuideCodeProps): JSX.Element {
  const {t} = useTranslation();
  const {copied, copy} = useCopyToClipboard();
  const selected = options.findIndex((option) => option.label === codeOption);
  const active = options[Math.max(selected, 0)];
  const segments = fillQuickstartGuideCode(active.code, codeValues);
  const filledCode = segments.map((segment) => segment.text).join('');
  const highlightedLanguage = active.language ? HIGHLIGHTED_LANGUAGES[active.language.toLowerCase()] : undefined;
  const codeLabel = title ?? active.label ?? t('applications:edit.overview.guide.codeSample', 'Code sample');
  const copyLabel = copied ? t('common:actions.copied', 'Copied!') : t('common:actions.copy', 'Copy');

  return (
    <Box sx={{border: 1, borderColor: 'divider', borderRadius: 1, overflow: 'hidden', minWidth: 0}}>
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        spacing={1}
        sx={{pl: options.length > 1 ? 0.5 : 2, pr: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'action.hover'}}
      >
        {options.length > 1 ? (
          <Tabs
            value={active.label}
            onChange={(_, label: string) => onCodeOptionChange?.(label)}
            variant="scrollable"
            scrollButtons="auto"
            aria-label={t('applications:edit.overview.guide.codeOptions', 'Code sample options')}
            // As compact as the header of a single-option sample, so a tabbed one is no taller.
            sx={{minHeight: 0}}
          >
            {options.map((option) => (
              <Tab
                key={option.label}
                value={option.label}
                label={option.label}
                icon={option.label ? OPTION_ICONS[option.label.toLowerCase()] : undefined}
                iconPosition="start"
                sx={{
                  minHeight: 0,
                  minWidth: 0,
                  px: 1.5,
                  py: 1,
                  gap: 0.75,
                  typography: 'caption',
                  textTransform: 'none',
                }}
              />
            ))}
          </Tabs>
        ) : (
          <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}} noWrap>
            {title ?? active.language}
          </Typography>
        )}
        <Tooltip title={copyLabel}>
          <IconButton
            size="small"
            aria-label={copyLabel}
            onClick={() => {
              void copy(filledCode);
            }}
          >
            {copied ? <Check size={16} aria-hidden /> : <Copy size={16} aria-hidden />}
          </IconButton>
        </Tooltip>
      </Stack>
      {highlightedLanguage ? (
        // Oxygen UI's code block colors the code. It brings its own frame, which this one already draws.
        <Box
          tabIndex={0}
          aria-label={codeLabel}
          sx={{
            '& .MuiBox-root': {border: 0, borderRadius: 0, mb: 0},
            // The card's own color and the plain samples' type, in place of the code block's fixed grey.
            '& .MuiBox-root pre': {bgcolor: 'transparent', typography: 'body2', fontFamily: 'monospace'},
          }}
        >
          <CodeBlock code={filledCode} language={highlightedLanguage} />
        </Box>
      ) : (
        <Box
          component="pre"
          tabIndex={0}
          aria-label={codeLabel}
          sx={{m: 0, p: 2, overflow: 'auto', typography: 'body2', fontFamily: 'monospace'}}
        >
          <code>
            {segments.map((segment, index) =>
              segment.filled ? (
                <Box
                  // eslint-disable-next-line react/no-array-index-key
                  key={index}
                  component="mark"
                  sx={(theme) => ({
                    color: 'inherit',
                    bgcolor: alpha(theme.palette.primary.main, 0.2),
                    borderRadius: 0.5,
                    px: 0.25,
                  })}
                >
                  {segment.text}
                </Box>
              ) : (
                segment.text
              ),
            )}
          </code>
        </Box>
      )}
    </Box>
  );
}
