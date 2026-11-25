// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Link, Stack, TextField, Typography, alpha} from '@wso2/oxygen-ui';
import {CircleX} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link as RouterLink} from 'react-router';
import useChangeName from '../hooks/useChangeName';
import useDeployRoutes, {resourceEditOf} from '../hooks/useDeployRoutes';
import {
  aboutOwner,
  aboutValues,
  fieldKey,
  refusalText,
  type MissingField,
  type RefusedFix,
} from '../utils/missingValues';

export interface RefusedFixesProps {
  fixes: RefusedFix[];
  /** Where the resources were refused. */
  target: string;
  /** Whether the check of the deploy has answered, so a resource it passes can be said to pass. */
  checked?: boolean;
  /** What is in each field: what was typed, else what the gateway holds. */
  valueOf: (field: MissingField) => string;
  onChange: (field: MissingField, value: string) => void;
  disabled?: boolean;
}

/**
 * The ticked resources the gateway refused last time or would refuse now, each with what the gateway
 * answered, what the check of the deploy says it would answer with the values it holds, and the
 * values it uses, filled with what the gateway holds, so a value that caused the refusal can be set
 * right before the deploy. A secret is never read back, so its field starts empty and is left as it
 * is unless something is typed.
 */
export default function RefusedFixes({
  fixes,
  target,
  checked = false,
  valueOf,
  onChange,
  disabled = false,
}: RefusedFixesProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const nameOfChange = useChangeName();
  const routes = useDeployRoutes();
  // The values of a refusal that values cannot fix, shown only when asked for.
  const [shown, setShown] = useState<ReadonlySet<string>>(new Set());
  return (
    <Stack
      spacing={2}
      data-testid="refused-fixes"
      sx={(theme) => ({
        p: 2,
        borderRadius: 2,
        border: 1,
        borderColor: alpha(theme.palette.error.main, 0.35),
        borderLeft: 4,
        borderLeftColor: 'error.main',
        bgcolor: alpha(theme.palette.error.main, 0.04),
      })}
    >
      <Stack direction="row" alignItems="center" spacing={1}>
        <Box sx={{display: 'flex', color: 'error.main'}}>
          <CircleX size={18} />
        </Box>
        <Typography variant="subtitle2">
          {fixes.some((fix: RefusedFix) => fix.predicted)
            ? t('refused.wouldRefuse', '{{target}} would refuse these', {target})
            : t('refused.title', 'Refused by {{target}} last time', {target})}
        </Typography>
      </Stack>
      {fixes.map(({change, failure, predicted, fields}: RefusedFix) => {
        const latest = predicted ?? failure;
        // A refusal values can fix offers its values; any other is fixed on the resource's own page.
        const byValues = !latest || aboutValues(latest, fields);
        const edit = change.id ? resourceEditOf(routes, change.resourceType, change.id) : undefined;
        const editTo = edit && aboutOwner(latest) && change.resourceType === 'agent' ? `${edit}?tab=advanced` : edit;
        const showFields = byValues || shown.has(change.key);
        const name = nameOfChange(change);
        return (
          <Stack key={change.key} spacing={1} data-testid={`refused-fix-${change.key}`}>
            <Box>
              <Typography variant="body2" fontWeight={600}>
                {name}
              </Typography>
              {predicted && (
                <Typography variant="caption" color="error.main" component="div" data-testid="refused-fix-predicted">
                  {t('refused.predicted', 'Would be refused: {{reason}}', {reason: refusalText(predicted)})}
                </Typography>
              )}
              {failure && (
                <Typography
                  variant="caption"
                  color={predicted ? 'text.secondary' : 'error.main'}
                  component="div"
                  data-testid="refused-fix-reason"
                >
                  {failure.message
                    ? t('refused.lastTime', 'Last time: {{reason}}', {reason: refusalText(failure)})
                    : t('refused.noReason', '{{target}} gave no reason.', {target})}
                </Typography>
              )}
              {failure && !predicted && checked && byValues && (
                <Typography variant="caption" color="success.main" component="div" data-testid="refused-fix-passes">
                  {t('refused.passesNow', 'Passes the check with the values {{target}} holds now.', {target})}
                </Typography>
              )}
              {!byValues && (
                <Typography
                  variant="caption"
                  color="text.secondary"
                  component="div"
                  data-testid="refused-fix-not-values"
                >
                  {t('refused.notValues', 'Values cannot fix this. Change {{name}} itself, then deploy again.', {name})}{' '}
                  {editTo && (
                    <Link component={RouterLink} to={editTo} data-testid="refused-fix-edit">
                      {t('refused.edit', 'Edit {{name}}', {name})}
                    </Link>
                  )}
                </Typography>
              )}
            </Box>
            {!showFields ? (
              fields.length > 0 && (
                <Button
                  variant="text"
                  size="small"
                  onClick={() => setShown((previous) => new Set(previous).add(change.key))}
                  sx={{alignSelf: 'flex-start', px: 0, minHeight: 0}}
                  data-testid="refused-fix-show-values"
                >
                  {t('refused.showValues', 'Show its values')}
                </Button>
              )
            ) : fields.length === 0 ? (
              <Typography variant="caption" color="text.secondary">
                {t(
                  'refused.noValues',
                  'It uses no values. Deploy it again, or fix what it needs on {{target}} first.',
                  {
                    target,
                  },
                )}
              </Typography>
            ) : (
              fields.map((field: MissingField) => {
                const id = `refused-${change.key}-${fieldKey(field)}`;
                const where = field.references
                  .map((reference) => reference.field)
                  .filter(Boolean)
                  .join(', ');
                return (
                  <Box key={fieldKey(field)}>
                    <Typography
                      component="label"
                      htmlFor={id}
                      variant="caption"
                      sx={{fontFamily: 'monospace', display: 'flex', gap: 0.75, mb: 0.5}}
                    >
                      {field.name}
                      {where && (
                        <Typography
                          component="span"
                          variant="caption"
                          color="text.secondary"
                          sx={{fontFamily: 'inherit'}}
                        >
                          {where}
                        </Typography>
                      )}
                    </Typography>
                    <TextField
                      id={id}
                      size="small"
                      fullWidth
                      type={field.kind === 'secret' ? 'password' : 'text'}
                      multiline={field.list}
                      minRows={field.list ? 2 : undefined}
                      autoComplete={field.kind === 'secret' ? 'new-password' : 'off'}
                      value={valueOf(field)}
                      onChange={(event) => onChange(field, event.target.value)}
                      disabled={disabled}
                      placeholder={
                        field.kind === 'secret' ? t('refused.secretPlaceholder', 'Set, type to replace') : undefined
                      }
                      helperText={field.list ? t('missing.listHint', 'One per line.') : undefined}
                      sx={{'& .MuiOutlinedInput-root': {bgcolor: 'background.paper'}}}
                    />
                  </Box>
                );
              })
            )}
          </Stack>
        );
      })}
    </Stack>
  );
}
