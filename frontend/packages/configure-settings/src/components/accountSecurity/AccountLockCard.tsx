// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Divider,
  FormControlLabel,
  Radio,
  RadioGroup,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DurationField from './DurationField';
import LockEmailRecipients from './LockEmailRecipients';
import LockLadderRows from './LockLadderRows';
import {type AccountLockCategory, type LockGranularity} from '../../models/accountAccess';
import type {AccountLockForm} from '../../utils/accountLockPayload';
import describePolicy from '../../utils/describePolicy';

/**
 * Props for {@link AccountLockCard}.
 *
 * @public
 */
export interface AccountLockCardProps {
  /** The identity category this card configures. */
  category: AccountLockCategory;
  /** The current form values. */
  form: AccountLockForm;
  /** Blocking messages, keyed by field. */
  errors: Partial<Record<keyof AccountLockForm, string>>;
  /** Called with the changed fields. */
  onChange: (patch: Partial<AccountLockForm>) => void;
  /** Whether the settings are read-only, as when they are managed declaratively. */
  disabled?: boolean;
}

/** Fallback copy for the per-category keys, matching en-US. */
const CATEGORY_FALLBACKS: Record<AccountLockCategory, {title: string; description: string; entityHint: string}> = {
  user: {
    title: 'Users',
    description: 'Temporarily restrict sign-in after repeated failed attempts.',
    entityHint: 'Prevent the user from starting a new sign-in using any method.',
  },
  agent: {
    title: 'Agents',
    description:
      'Temporarily restrict authentication after repeated failed attempts. An agent client secret is never locked.',
    entityHint: 'Prevent the agent from authenticating using any method.',
  },
};

/**
 * The automatic-lock settings for one identity category: a policy summary, the threshold and
 * granularity, and durations, reset and email under *Advanced settings*. A validation error inside
 * the advanced section holds it open. Switching the card off hides every setting.
 *
 * @param props - The category, its values, the change handler, and whether it is read-only
 * @returns The rendered card
 *
 * @public
 */
export default function AccountLockCard({
  category,
  form,
  errors,
  onChange,
  disabled = false,
}: AccountLockCardProps): JSX.Element {
  const {t} = useTranslation();

  // Resolves a bare key in the settings catalog, which is what the summary helper expects.
  const tSettings = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(`settings:${key}`, options ?? {}),
    [t],
  );

  const summary: string = describePolicy(form, tSettings);

  // The reset only acts on an escalating ladder, but stays visible for a value already set. "Never
  // reset" is offered only while escalating, since its zero would hide this section on one rung.
  const escalating: boolean = form.lockDurationsSeconds.length > 1;
  const showDecay: boolean = escalating || form.lockDecaySeconds > 0;

  // Collapsed by default; an error inside pins it open.
  const [expanded, setExpanded] = useState<boolean>(false);
  const hasAdvancedError: boolean =
    Boolean(errors.lockDurationsSeconds) ||
    Boolean(errors.lockDecaySeconds) ||
    Boolean(errors.notifyRecipientAttribute);

  const title: string = t(`settings:accountLock.${category}.title`, CATEGORY_FALLBACKS[category].title);

  return (
    <Box
      component="section"
      aria-label={title}
      data-testid={`account-lock-${category}`}
      sx={{border: 1, borderColor: 'divider', borderRadius: 1, p: 3}}
    >
      <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
        <Typography variant="h6">{title}</Typography>
        <Switch
          checked={form.enabled}
          disabled={disabled}
          onChange={(event) => onChange({enabled: event.target.checked})}
          // Overriding slotProps.input drops MUI's default switch role, so restate it.
          slotProps={{
            input: {
              role: 'switch',
              'aria-label': t('settings:accountLock.toggle.ariaLabel', 'Automatic lock for {{title}}', {title}),
            },
          }}
        />
      </Stack>
      <Typography variant="body2" sx={{mt: 0.5, color: 'text.secondary'}}>
        {t(`settings:accountLock.${category}.description`, CATEGORY_FALLBACKS[category].description)}
      </Typography>
      {form.enabled && (
        <Stack spacing={3} sx={{mt: 3}}>
          {summary && (
            <Box
              data-testid={`policy-summary-${category}`}
              sx={{
                backgroundColor: 'action.hover',
                borderRadius: 1,
                px: 2,
                py: 1.5,
              }}
            >
              <Typography variant="body2">{summary}</Typography>
            </Box>
          )}

          <Box>
            <Typography variant="subtitle2" sx={{mb: 1}}>
              {t('settings:accountLock.threshold.label', 'Lock after')}
            </Typography>
            <Stack direction="row" spacing={1} useFlexGap alignItems="flex-start" sx={{flexWrap: 'wrap'}}>
              <TextField
                type="number"
                size="small"
                aria-label={t('settings:accountLock.threshold.ariaLabel', 'Failed attempts before lock')}
                value={form.threshold === 0 ? '' : String(form.threshold)}
                disabled={disabled}
                error={Boolean(errors.threshold)}
                helperText={errors.threshold}
                slotProps={{
                  htmlInput: {
                    min: 1,
                    'aria-label': t('settings:accountLock.threshold.ariaLabel', 'Failed attempts before lock'),
                    'data-testid': `threshold-${category}`,
                  },
                }}
                onChange={(event) => onChange({threshold: Number(event.target.value)})}
                sx={{width: 96, '& .MuiInputBase-root': {height: 40}}}
              />
              <Typography variant="body2" sx={{pt: 1.25}}>
                {t('settings:accountLock.threshold.within', 'failed attempts within')}
              </Typography>
              <DurationField
                inline
                label={t('settings:accountLock.window.ariaLabel', 'Period the failed attempts are counted over')}
                seconds={form.failureWindowSeconds}
                error={errors.failureWindowSeconds}
                zeroLabel={t('settings:accountLock.window.never', 'No time limit')}
                testId={`window-${category}`}
                disabled={disabled}
                onChange={(failureWindowSeconds) => onChange({failureWindowSeconds})}
              />
            </Stack>
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle2" sx={{mb: 0.5}}>
              {t('settings:accountLock.granularity.label', 'Lock applies to')}
            </Typography>
            <RadioGroup
              value={form.granularity}
              onChange={(event) => onChange({granularity: event.target.value as LockGranularity})}
            >
              <FormControlLabel
                value="authentication_method"
                disabled={disabled}
                control={<Radio size="small" data-testid={`granularity-authentication-method-${category}`} />}
                label={
                  <Box sx={{py: 0.5}}>
                    <Typography variant="body2">
                      {t(
                        'settings:accountLock.granularity.authenticationMethod',
                        'Only the sign-in method that failed',
                      )}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        'settings:accountLock.granularity.authenticationMethodHint',
                        'Recommended. Other sign-in methods remain available.',
                      )}
                    </Typography>
                  </Box>
                }
              />
              <FormControlLabel
                value="entity"
                disabled={disabled}
                control={<Radio size="small" data-testid={`granularity-entity-${category}`} />}
                label={
                  <Box sx={{py: 0.5}}>
                    <Typography variant="body2">
                      {t('settings:accountLock.granularity.entity', 'Every sign-in method')}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        `settings:accountLock.granularity.entityHint.${category}`,
                        CATEGORY_FALLBACKS[category].entityHint,
                      )}
                    </Typography>
                  </Box>
                }
              />
            </RadioGroup>
          </Box>

          <Divider />

          <Accordion
            disableGutters
            square
            expanded={expanded || hasAdvancedError}
            onChange={(_event, isExpanded: boolean) => setExpanded(isExpanded)}
            slotProps={{transition: {unmountOnExit: true}}}
            sx={{bgcolor: 'transparent', boxShadow: 'none', border: 'none', '&:before': {display: 'none'}}}
          >
            <AccordionSummary
              expandIcon={<ChevronDown size={16} />}
              data-testid={`advanced-${category}`}
              sx={{
                px: 0,
                minHeight: 'auto',
                bgcolor: 'transparent',
                '&.Mui-expanded': {minHeight: 'auto', bgcolor: 'transparent'},
                '& .MuiAccordionSummary-content': {my: 1},
              }}
            >
              <Typography variant="subtitle2">
                {t('settings:accountLock.advanced.label', 'Advanced settings')}
              </Typography>
            </AccordionSummary>
            <AccordionDetails sx={{px: 0, pt: 2}}>
              <Stack spacing={3} divider={<Divider />}>
                <LockLadderRows
                  durations={form.lockDurationsSeconds}
                  error={errors.lockDurationsSeconds}
                  disabled={disabled}
                  onChange={(lockDurationsSeconds) => onChange({lockDurationsSeconds})}
                />

                {showDecay && (
                  <Box>
                    <Typography variant="subtitle2" sx={{mb: 0.5}}>
                      {t('settings:accountLock.decay.label', 'Reset lock progression')}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                      {t(
                        'settings:accountLock.decay.hint',
                        'Return to the first lock duration after this period passes without another lock.',
                      )}
                    </Typography>
                    <DurationField
                      inline
                      label={t(
                        'settings:accountLock.decay.ariaLabel',
                        'Period without a lock before the progression resets',
                      )}
                      seconds={form.lockDecaySeconds}
                      error={errors.lockDecaySeconds}
                      zeroLabel={escalating ? t('settings:accountLock.decay.never', 'Never reset') : undefined}
                      testId={`decay-${category}`}
                      disabled={disabled}
                      onChange={(lockDecaySeconds) => onChange({lockDecaySeconds})}
                    />
                  </Box>
                )}

                {category === 'user' && (
                  <Box>
                    <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={2}>
                      <Typography variant="subtitle2">
                        {t('settings:accountLock.notify.label', 'Email the account holder')}
                      </Typography>
                      <Switch
                        checked={form.notifyEnabled}
                        disabled={disabled}
                        data-testid={`notify-${category}`}
                        onChange={(event) => onChange({notifyEnabled: event.target.checked})}
                        slotProps={{
                          input: {
                            role: 'switch',
                            'aria-label': t('settings:accountLock.notify.label', 'Email the account holder'),
                          },
                        }}
                      />
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        'settings:accountLock.notify.hint',
                        'Send an email notification when a sign-in method is locked.',
                      )}
                    </Typography>
                    {(form.notifyEnabled || Boolean(errors.notifyRecipientAttribute)) && (
                      <Box sx={{mt: 2, maxWidth: 400}}>
                        <LockEmailRecipients form={form} errors={errors} onChange={onChange} disabled={disabled} />
                      </Box>
                    )}
                  </Box>
                )}
              </Stack>
            </AccordionDetails>
          </Accordion>
        </Stack>
      )}
    </Box>
  );
}
