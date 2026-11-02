// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Checkbox,
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
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import DurationField from './DurationField';
import LockLadderRows from './LockLadderRows';
import {LockGranularities, type AccountLockCategory, type LockGranularity} from '../../models/accountAccess';
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
const CATEGORY_FALLBACKS: Record<
  AccountLockCategory,
  {
    title: string;
    description: string;
    methodLabel: string;
    methodHint: string;
    accountLabel: string;
    accountHint: string;
    ceilingHint: string;
  }
> = {
  user: {
    title: 'Users',
    description: 'Temporarily block sign-in after repeated failed attempts.',
    methodLabel: 'Only the affected sign-in method',
    methodHint: 'Users can still sign in using other methods.',
    accountLabel: 'All sign-in methods',
    accountHint: 'Prevent users from signing in using any method until the lockout ends.',
    ceilingHint:
      'Require account recovery or administrator intervention when the number of consecutive failed sign-in attempts reaches this limit.',
  },
  agent: {
    title: 'Agents',
    description:
      'Temporarily block agent authentication after repeated failed attempts. Client secret authentication is excluded from lockouts.',
    methodLabel: 'Only the affected authentication method',
    methodHint: 'Agents can still authenticate using other methods.',
    accountLabel: 'All authentication methods',
    accountHint: 'Prevent agents from authenticating using any method until the lockout ends.',
    ceilingHint:
      'Require administrator intervention when the number of consecutive failed authentication attempts reaches this limit.',
  },
};

/**
 * The automatic-lock settings for one identity category: a policy summary, the threshold and
 * granularity, and durations, reset and email under *Advanced settings*. A validation error inside
 * the advanced section holds it open. Switching the card off hides every setting, and the summary
 * then says that failed attempts lock nothing.
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

  const summary: string = describePolicy(form, category, t);

  // The reset only acts on an escalating ladder, but stays visible for a value already set. "Never
  // reset" is offered only while escalating, since its zero would hide this section on one rung.
  const escalating: boolean = form.lockDurationsSeconds.length > 1;
  const showDecay: boolean = escalating || form.lockDecaySeconds > 0;

  // Collapsed by default; an error inside pins it open.
  const [expanded, setExpanded] = useState<boolean>(false);
  const hasAdvancedError: boolean =
    Boolean(errors.lockDurationsSeconds) || Boolean(errors.lockDecaySeconds) || Boolean(errors.maxConsecutiveFailures);

  const title: string = t(`settings:accountLock.${category}.title`, CATEGORY_FALLBACKS[category].title);
  const granularityLabel: string = t('settings:accountLock.granularity.label', 'Lockout applies to');

  const summaryBox: JSX.Element | null = summary ? (
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
  ) : null;

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
      {!form.enabled && <Box sx={{mt: 3}}>{summaryBox}</Box>}
      {form.enabled && (
        <Stack spacing={3} sx={{mt: 3}}>
          {summaryBox}

          <Box>
            <Typography variant="subtitle2" sx={{mb: 1}}>
              {t('settings:accountLock.threshold.label', 'Lock after')}
            </Typography>
            <Stack direction="row" spacing={1} useFlexGap alignItems="flex-start" sx={{flexWrap: 'wrap'}}>
              <TextField
                type="number"
                size="small"
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
                label={t('settings:accountLock.window.ariaLabel', 'Longest gap allowed between failed attempts')}
                seconds={form.failureWindowSeconds}
                error={errors.failureWindowSeconds}
                zeroLabel={t('settings:accountLock.window.never', 'No time limit')}
                testId={`window-${category}`}
                disabled={disabled}
                onChange={(failureWindowSeconds) => onChange({failureWindowSeconds})}
              />
            </Stack>
            {/* The window is the longest gap between failures, not a fixed period. */}
            <Typography variant="body2" color="text.secondary" sx={{mt: 1}}>
              {t(
                'settings:accountLock.window.hint',
                'A longer gap between two failed attempts starts the count again.',
              )}
            </Typography>
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle2" sx={{mb: 0.5}}>
              {granularityLabel}
            </Typography>
            <RadioGroup
              aria-label={granularityLabel}
              value={form.granularity}
              onChange={(event) => onChange({granularity: event.target.value as LockGranularity})}
            >
              <FormControlLabel
                value={LockGranularities.AUTHENTICATION_METHOD}
                disabled={disabled}
                control={<Radio size="small" data-testid={`granularity-authentication-method-${category}`} />}
                label={
                  <Box sx={{py: 0.5}}>
                    <Typography variant="body2">
                      {t(
                        `settings:accountLock.granularity.authenticationMethod.${category}`,
                        CATEGORY_FALLBACKS[category].methodLabel,
                      )}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        `settings:accountLock.granularity.authenticationMethodHint.${category}`,
                        CATEGORY_FALLBACKS[category].methodHint,
                      )}
                    </Typography>
                  </Box>
                }
              />
              <FormControlLabel
                value={LockGranularities.ACCOUNT}
                disabled={disabled}
                control={<Radio size="small" data-testid={`granularity-account-${category}`} />}
                label={
                  <Box sx={{py: 0.5}}>
                    <Typography variant="body2">
                      {t(
                        `settings:accountLock.granularity.account.${category}`,
                        CATEGORY_FALLBACKS[category].accountLabel,
                      )}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(
                        `settings:accountLock.granularity.accountHint.${category}`,
                        CATEGORY_FALLBACKS[category].accountHint,
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
                  category={category}
                  durations={form.lockDurationsSeconds}
                  error={errors.lockDurationsSeconds}
                  disabled={disabled}
                  onChange={(lockDurationsSeconds) => onChange({lockDurationsSeconds})}
                />

                {showDecay && (
                  <Box>
                    <Typography variant="subtitle2" sx={{mb: 0.5}}>
                      {t('settings:accountLock.decay.label', 'Reset lockout progression')}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                      {t(
                        'settings:accountLock.decay.hint',
                        'Restart the lockout sequence if no new lockout occurs within this period after the last one ends.',
                      )}
                    </Typography>
                    <DurationField
                      inline
                      label={t(
                        'settings:accountLock.decay.ariaLabel',
                        'Period without a lockout before the sequence restarts',
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

                <Box>
                  <Typography variant="subtitle2" sx={{mb: 0.5}}>
                    {t('settings:accountLock.ceiling.label', 'Consecutive failure limit')}
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{mb: 1.5}}>
                    {t(`settings:accountLock.ceiling.hint.${category}`, CATEGORY_FALLBACKS[category].ceilingHint)}
                  </Typography>
                  <TextField
                    type="number"
                    size="small"
                    value={String(form.maxConsecutiveFailures)}
                    disabled={disabled}
                    error={Boolean(errors.maxConsecutiveFailures)}
                    helperText={errors.maxConsecutiveFailures}
                    slotProps={{
                      htmlInput: {
                        min: 0,
                        'aria-label': t('settings:accountLock.ceiling.ariaLabel', 'Consecutive failure limit'),
                        'data-testid': `ceiling-${category}`,
                      },
                    }}
                    onChange={(event) => onChange({maxConsecutiveFailures: Number(event.target.value)})}
                    sx={{width: 96, '& .MuiInputBase-root': {height: 40}}}
                  />
                </Box>

                {category === 'user' && (
                  <Box>
                    <FormControlLabel
                      disabled={disabled}
                      control={
                        <Checkbox
                          size="small"
                          checked={form.notifyEnabled}
                          data-testid={`notify-${category}`}
                          onChange={(event) => onChange({notifyEnabled: event.target.checked})}
                          slotProps={{input: {'aria-describedby': `notify-hint-${category}`}}}
                        />
                      }
                      label={
                        <Typography variant="subtitle2">
                          {t('settings:accountLock.notify.label', 'Notify user by email')}
                        </Typography>
                      }
                      sx={{mr: 0}}
                    />
                    <Typography
                      id={`notify-hint-${category}`}
                      variant="caption"
                      color="text.secondary"
                      sx={{display: 'block'}}
                    >
                      {t(
                        'settings:accountLock.notify.hint',
                        "Send an email to the user's primary email address when a sign-in method is locked. Users without a primary email address won't receive notifications.",
                      )}
                    </Typography>
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
