// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, CardActionArea, CardContent, InputAdornment, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Link2, Mail, MessageSquare, Plus, Search, ShieldCheck, UserPlus} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {getSampleTemplates, type SampleNotificationTemplate} from '../../constants/sample-templates';
import type {NotificationChannel} from '../../models/notification-template';

/**
 * The create wizard's starting-point choice: either an empty template ("start from scratch") or one
 * of the hardcoded {@link SampleNotificationTemplate}s for the channel.
 *
 * @public
 */
export type TemplateSelection = {kind: 'blank'} | {kind: 'sample'; sample: SampleNotificationTemplate};

/**
 * Props for the {@link SelectNotificationTemplate} component.
 *
 * @public
 */
export interface SelectNotificationTemplateProps {
  /** The channel whose samples are offered. */
  channel: NotificationChannel;
  /** The current selection, or null when nothing is chosen yet. */
  selected: TemplateSelection | null;
  /** Called when the user picks the blank option or a sample. */
  onSelect: (selection: TemplateSelection) => void;
}

/** Icon shown on a sample card, chosen by sample id with a per-channel fallback. */
function sampleIcon(sample: SampleNotificationTemplate): ReactNode {
  if (sample.id.includes('otp')) return <KeyRound size={22} />;
  if (sample.id.includes('magic-link')) return <Link2 size={22} />;
  if (sample.id.includes('invite') || sample.id.includes('registration')) return <UserPlus size={22} />;
  if (sample.id.includes('password')) return <ShieldCheck size={22} />;
  return sample.channel === 'email' ? <Mail size={22} /> : <MessageSquare size={22} />;
}

/**
 * Step that lets the user start a template from scratch or from a sample, mirroring the flow
 * builder's "Choose a starting template" step. Samples are hardcoded per channel.
 *
 * @public
 */
export default function SelectNotificationTemplate({
  channel,
  selected,
  onSelect,
}: SelectNotificationTemplateProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const [search, setSearch] = useState('');

  const samples = useMemo(() => getSampleTemplates(channel), [channel]);
  const query = search.trim().toLowerCase();
  const filtered = useMemo(
    () =>
      query
        ? samples.filter(
            (sample) =>
              sample.display.label.toLowerCase().includes(query) ||
              sample.display.description.toLowerCase().includes(query),
          )
        : samples,
    [samples, query],
  );

  const isBlankSelected = selected?.kind === 'blank';

  return (
    <Stack direction="column" spacing={3} data-testid="select-notification-template">
      <Typography variant="h1">{t('create.template.title', 'Choose a starting template')}</Typography>

      {/* Start from scratch */}
      <Card
        variant="outlined"
        sx={{
          borderWidth: 2,
          borderStyle: isBlankSelected ? 'solid' : 'dashed',
          borderColor: isBlankSelected ? 'primary.main' : 'divider',
          borderRadius: 2,
          bgcolor: isBlankSelected ? 'action.selected' : 'transparent',
          transition: 'all 0.15s ease-in-out',
          '&:hover': {borderColor: 'primary.main', bgcolor: isBlankSelected ? 'action.selected' : 'action.hover'},
        }}
      >
        <CardActionArea onClick={() => onSelect({kind: 'blank'})} data-testid="template-card-blank">
          <CardContent sx={{py: 2, px: 2.5}}>
            <Stack direction="row" spacing={2} alignItems="center">
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  width: 36,
                  height: 36,
                  borderRadius: 1.5,
                  bgcolor: isBlankSelected ? 'primary.main' : 'action.hover',
                  color: isBlankSelected ? 'primary.contrastText' : 'text.secondary',
                  transition: 'all 0.15s ease-in-out',
                }}
              >
                <Plus size={18} />
              </Box>
              <Box>
                <Typography variant="subtitle2" sx={{fontWeight: 600}}>
                  {t('create.template.blank.title', 'Start from scratch')}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {t('create.template.blank.description', 'Build the template from an empty subject and body.')}
                </Typography>
              </Box>
            </Stack>
          </CardContent>
        </CardActionArea>
      </Card>

      {/* Search */}
      <TextField
        size="small"
        placeholder={t('create.template.search', 'Search templates...')}
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        sx={{width: 240}}
        slotProps={{
          input: {
            startAdornment: (
              <InputAdornment position="start">
                <Search size={16} />
              </InputAdornment>
            ),
          },
        }}
      />

      {/* Sample grid */}
      {filtered.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          {t('create.template.noResults', 'No templates match your search.')}
        </Typography>
      ) : (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)', lg: 'repeat(4, 1fr)'},
            gap: 2,
          }}
        >
          {filtered.map((sample) => {
            const isSelected = selected?.kind === 'sample' && selected.sample.id === sample.id;
            return (
              <Card
                key={sample.id}
                variant="outlined"
                role="button"
                tabIndex={0}
                aria-pressed={isSelected}
                data-testid={`template-card-${sample.id}`}
                onClick={() => onSelect({kind: 'sample', sample})}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onSelect({kind: 'sample', sample});
                  }
                }}
                sx={{
                  cursor: 'pointer',
                  borderColor: isSelected ? 'primary.main' : 'divider',
                  bgcolor: isSelected ? 'action.selected' : undefined,
                  transition: 'border-color 0.15s',
                  '&:hover': {borderColor: 'primary.main'},
                  '&:focus-visible': {outline: 'none', borderColor: 'primary.main'},
                }}
              >
                <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                  <Stack direction="column" spacing={2}>
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 44,
                        height: 44,
                        borderRadius: 1.5,
                        bgcolor: 'action.hover',
                        color: 'primary.main',
                      }}
                    >
                      {sampleIcon(sample)}
                    </Box>
                    <Stack direction="column" spacing={0.75}>
                      <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                        {sample.display.label}
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                        {sample.display.description}
                      </Typography>
                    </Stack>
                  </Stack>
                </CardContent>
              </Card>
            );
          })}
        </Box>
      )}
    </Stack>
  );
}
