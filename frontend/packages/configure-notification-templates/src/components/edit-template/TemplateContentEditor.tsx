// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {templateWarningMessage, type TemplateWarning} from '@thunderid/configure-design';
import {
  Box,
  Card,
  CardContent,
  FormHelperText,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {Moon, Sun} from '@wso2/oxygen-ui-icons-react';
import {useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import TokenCatalog from './TokenCatalog';
import type {NotificationChannel, TemplateColorScheme} from '../../models/notification-template';

// Links the body input to its warnings, which render outside the TextField (see below).
const BODY_WARNINGS_ID = 'template-body-warnings';

/**
 * Props for the {@link TemplateContentEditor} component.
 *
 * @public
 */
export interface TemplateContentEditorProps {
  channel: NotificationChannel;
  subject: string;
  body: string;
  colorScheme: TemplateColorScheme;
  onSubjectChange: (value: string) => void;
  onBodyChange: (value: string) => void;
  onColorSchemeChange: (value: TemplateColorScheme) => void;
  /** Token warnings from the preview; each is shown on the field it came from. */
  warnings?: TemplateWarning[];
  /** Wrap the fields in an outlined card. Defaults to true. */
  card?: boolean;
}

/**
 * The template config pane: a color-scheme control (email), the subject (email) and body fields,
 * and an insertable token catalog alongside the body. Shared by the edit page and the create
 * wizard's content step; the preview is rendered separately by the caller.
 *
 * @public
 */
export default function TemplateContentEditor({
  channel,
  subject,
  body,
  colorScheme,
  onSubjectChange,
  onBodyChange,
  onColorSchemeChange,
  warnings = [],
  card = true,
}: TemplateContentEditorProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const isEmail = channel === 'email';

  const subjectWarnings = warnings.filter((warning) => warning.field === 'subject');
  const bodyWarnings = warnings.filter((warning) => warning.field === 'body');
  const subjectHasError = subjectWarnings.some((warning) => warning.severity === 'error');
  const bodyHasError = bodyWarnings.some((warning) => warning.severity === 'error');
  const bodyBorderColor = bodyHasError ? 'error.main' : bodyWarnings.length > 0 ? 'warning.main' : 'divider';

  // One line per warning, colored by its own severity so a warning listed alongside errors stays amber.
  const renderWarnings = (fieldWarnings: TemplateWarning[]): JSX.Element[] =>
    fieldWarnings.map((warning) => (
      <Box
        component="span"
        key={`${warning.code}:${warning.token}`}
        sx={{display: 'block', color: warning.severity === 'error' ? 'error.main' : 'warning.main'}}
      >
        {templateWarningMessage(t, warning)}
      </Box>
    ));

  const [tokensCollapsed, setTokensCollapsed] = useState(false);
  const subjectRef = useRef<HTMLInputElement>(null);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const activeField = useRef<'subject' | 'body'>('body');

  const insertToken = (value: string): void => {
    const field = isEmail && activeField.current === 'subject' ? 'subject' : 'body';
    const el = field === 'subject' ? subjectRef.current : bodyRef.current;
    const current = field === 'subject' ? subject : body;
    const start = el?.selectionStart ?? current.length;
    const end = el?.selectionEnd ?? current.length;
    const next = current.slice(0, start) + value + current.slice(end);
    if (field === 'subject') {
      onSubjectChange(next);
    } else {
      onBodyChange(next);
    }
    const caret = start + value.length;
    requestAnimationFrame(() => {
      if (el) {
        el.focus();
        el.setSelectionRange(caret, caret);
      }
    });
  };

  const content = (
    <>
      {isEmail && (
        <Box sx={{mb: 3, p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 2}}>
          <Box
            sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 2, flexWrap: 'wrap'}}
          >
            <Box sx={{flex: '1 1 220px', minWidth: 0}}>
              <Typography variant="overline" sx={{color: 'text.secondary', fontWeight: 700}}>
                {t('editor.design.title', 'Design')}
              </Typography>
              <Typography variant="caption" sx={{display: 'block', color: 'text.secondary'}}>
                {t(
                  'editor.design.help',
                  'Choose the color theme for this email. The preview uses the selected design theme.',
                )}
              </Typography>
            </Box>
            <Box>
              <Typography variant="body2" sx={{mb: 0.75, color: 'text.secondary', fontWeight: 500}}>
                {t('editor.design.colorScheme', 'Color scheme')}
              </Typography>
              <ToggleButtonGroup
                value={colorScheme}
                exclusive
                size="small"
                onChange={(_, value: TemplateColorScheme | null) => {
                  if (value) onColorSchemeChange(value);
                }}
              >
                <ToggleButton value="light" sx={{gap: 0.75, px: 2, textTransform: 'capitalize', fontSize: '0.75rem'}}>
                  <Sun size={14} />
                  {t('editor.design.light', 'Light')}
                </ToggleButton>
                <ToggleButton value="dark" sx={{gap: 0.75, px: 2, textTransform: 'capitalize', fontSize: '0.75rem'}}>
                  <Moon size={14} />
                  {t('editor.design.dark', 'Dark')}
                </ToggleButton>
              </ToggleButtonGroup>
            </Box>
          </Box>
        </Box>
      )}

      {isEmail && (
        <>
          <Typography variant="body2" sx={{mb: 1, color: 'text.secondary', fontWeight: 500}}>
            {t('editor.subject', 'Subject')}
          </Typography>
          <TextField
            fullWidth
            value={subject}
            onChange={(e) => onSubjectChange(e.target.value)}
            onFocus={() => {
              activeField.current = 'subject';
            }}
            inputRef={subjectRef}
            error={subjectHasError}
            helperText={subjectWarnings.length > 0 ? renderWarnings(subjectWarnings) : undefined}
            sx={{mb: 2.5}}
          />
        </>
      )}

      <Typography variant="body2" sx={{mb: 1, color: 'text.secondary', fontWeight: 500}}>
        {t('editor.body', 'Body')}
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: tokensCollapsed ? 'minmax(0, 1fr) auto' : 'minmax(0, 1fr) 285px',
          border: '1px solid',
          borderColor: bodyBorderColor,
          borderRadius: 1,
          overflow: 'hidden',
          minHeight: 440,
        }}
      >
        <TextField
          multiline
          minRows={18}
          value={body}
          onChange={(e) => onBodyChange(e.target.value)}
          onFocus={() => {
            activeField.current = 'body';
          }}
          inputRef={bodyRef}
          error={bodyHasError}
          inputProps={{'aria-describedby': bodyWarnings.length > 0 ? BODY_WARNINGS_ID : undefined}}
          InputProps={{sx: {fontFamily: 'monospace', fontSize: '0.8125rem', alignItems: 'flex-start', height: '100%'}}}
          sx={{
            height: '100%',
            '& .MuiOutlinedInput-root': {height: '100%', borderRadius: 0, alignItems: 'flex-start'},
            '& .MuiOutlinedInput-notchedOutline': {border: 0},
          }}
        />
        <TokenCatalog
          channel={channel}
          onInsert={insertToken}
          collapsed={tokensCollapsed}
          onToggleCollapse={() => setTokensCollapsed((c) => !c)}
        />
      </Box>
      {/* The body's own outline is hidden inside the shared box above, so its warnings render here. */}
      {bodyWarnings.length > 0 && <FormHelperText id={BODY_WARNINGS_ID}>{renderWarnings(bodyWarnings)}</FormHelperText>}
    </>
  );

  if (!card) {
    return content;
  }

  return (
    <Card variant="outlined" sx={{borderRadius: 2}}>
      <CardContent>{content}</CardContent>
    </Card>
  );
}
