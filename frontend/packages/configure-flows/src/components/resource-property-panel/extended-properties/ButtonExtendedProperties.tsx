// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Divider, FormHelperText, FormLabel, MenuItem, Select, Stack, TextField} from '@wso2/oxygen-ui';
import {useState, type ReactNode, type ChangeEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {Element} from '../../../models/elements';
import {ActionEventTypes, PromptActionTypes} from '../../../models/elements';
import type {CommonResourcePropertiesPropsInterface} from '../CommonResourceProperties';

/**
 * The options offered by the Action selector. `Submit` and `Trigger` are the
 * button's `eventType`; `Confirm` and `Reject` are submit buttons that
 * additionally raise a prompt action, so one selection maps onto two fields.
 *
 * The prompt actions deliberately carry the same literals that are persisted as
 * `prompts[].action.type`, so the value selected here and the value in the flow
 * definition are one vocabulary rather than a UI-only alias. Neither is specific
 * to a use case: the session sign-out executor reads `CONFIRM` and the account
 * linking executor reads `REJECT`, but any executor routing to such a prompt can.
 *
 * The remaining `ActionEventTypes` (navigate, cancel, reset, back) are handled
 * by the SDK renderers but deliberately not offered here.
 */
const ACTION_OPTIONS = {
  Submit: 'SUBMIT',
  Trigger: 'TRIGGER',
  Confirm: PromptActionTypes.Confirm,
  Reject: PromptActionTypes.Reject,
} as const;

type ActionOption = (typeof ACTION_OPTIONS)[keyof typeof ACTION_OPTIONS];

/** The options that are a submit button plus a prompt action. */
const PROMPT_ACTION_OPTIONS = new Set<string>([ACTION_OPTIONS.Confirm, ACTION_OPTIONS.Reject]);

/**
 * Props interface of {@link ButtonExtendedProperties}
 */
export type ButtonExtendedPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

/**
 * Extended properties for the button elements.
 * Provides optional start icon and end icon configuration.
 *
 * @param props - Props injected to the component.
 * @returns The ButtonExtendedProperties component.
 */
function ButtonExtendedProperties({resource, onChange}: ButtonExtendedPropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const element = resource as Element & {eventType?: string};
  const eventTypeValue = element?.eventType ?? ActionEventTypes.Trigger;

  // A prompt action is carried by a submit button, so it takes precedence over
  // the plain event type when deriving the selection.
  const hasPromptAction = PROMPT_ACTION_OPTIONS.has(element?.actionType ?? '');
  const actionValue: ActionOption = hasPromptAction
    ? (element.actionType as ActionOption)
    : eventTypeValue === ActionEventTypes.Submit
      ? ACTION_OPTIONS.Submit
      : ACTION_OPTIONS.Trigger;

  const handleActionChange = (nextAction: ActionOption): void => {
    if (PROMPT_ACTION_OPTIONS.has(nextAction)) {
      onChange('eventType', ActionEventTypes.Submit, resource);
      onChange('actionType', nextAction, resource);
      return;
    }

    onChange('eventType', nextAction, resource);
    // Clearing keeps the button from silently staying a prompt action after the
    // author picks a plain action. Only the types this selector owns are cleared,
    // so one it does not model, authored in the flow definition directly, is left
    // untouched rather than discarded.
    if (hasPromptAction) {
      onChange('actionType', '', resource);
    }
  };

  // Use local state for text inputs — provides immediate keystroke feedback while onChange is debounced
  const [startIconValue, setStartIconValue] = useState(() => {
    const element = resource as Element & {startIcon?: string};
    return element?.startIcon ?? '';
  });

  const [endIconValue, setEndIconValue] = useState(() => {
    const element = resource as Element & {endIcon?: string};
    return element?.endIcon ?? '';
  });

  // Sync local state when resource changes (e.g., switching to a different button)
  const [prevResource, setPrevResource] = useState(resource);
  if (resource !== prevResource) {
    setPrevResource(resource);
    const element = resource as Element & {startIcon?: string; endIcon?: string};
    setStartIconValue(element?.startIcon ?? '');
    setEndIconValue(element?.endIcon ?? '');
  }

  // Handle startIcon change - update local state immediately, propagate via onChange (debounced)
  const handleStartIconChange = (value: string): void => {
    setStartIconValue(value);
    onChange('startIcon', value, resource, true);
  };

  // Handle endIcon change - update local state immediately, propagate via onChange (debounced)
  const handleEndIconChange = (value: string): void => {
    setEndIconValue(value);
    onChange('endIcon', value, resource, true);
  };

  return (
    <Stack gap={2}>
      <Divider sx={{marginY: 2}} />

      <div>
        <FormLabel htmlFor="event-type-select">
          {t('flows:core.buttonExtendedProperties.action.label', 'Action')}
        </FormLabel>
        <Select
          id="event-type-select"
          value={actionValue}
          onChange={(e) => handleActionChange(e.target.value as ActionOption)}
          fullWidth
          size="small"
        >
          <MenuItem value={ACTION_OPTIONS.Submit}>
            {t('flows:core.buttonExtendedProperties.action.submit', 'Submit Form')}
          </MenuItem>
          <MenuItem value={ACTION_OPTIONS.Trigger}>
            {t('flows:core.buttonExtendedProperties.action.trigger', 'Trigger Action')}
          </MenuItem>
          <MenuItem value={ACTION_OPTIONS.Confirm}>
            {t('flows:core.buttonExtendedProperties.action.confirm', 'Confirm Action')}
          </MenuItem>
          <MenuItem value={ACTION_OPTIONS.Reject}>
            {t('flows:core.buttonExtendedProperties.action.reject', 'Reject Action')}
          </MenuItem>
        </Select>
        <FormHelperText>
          {t('flows:core.buttonExtendedProperties.action.hint', 'What happens when the button is activated')}
        </FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="start-icon-input">{t('flows:core.buttonExtendedProperties.startIcon.label')}</FormLabel>
        <TextField
          id="start-icon-input"
          value={startIconValue}
          onChange={(e: ChangeEvent<HTMLInputElement>) => handleStartIconChange(e.target.value)}
          placeholder={t('flows:core.buttonExtendedProperties.startIcon.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.buttonExtendedProperties.startIcon.hint')}</FormHelperText>
      </div>

      <div>
        <FormLabel htmlFor="end-icon-input">{t('flows:core.buttonExtendedProperties.endIcon.label')}</FormLabel>
        <TextField
          id="end-icon-input"
          value={endIconValue}
          onChange={(e: ChangeEvent<HTMLInputElement>) => handleEndIconChange(e.target.value)}
          placeholder={t('flows:core.buttonExtendedProperties.endIcon.placeholder')}
          fullWidth
          size="small"
        />
        <FormHelperText>{t('flows:core.buttonExtendedProperties.endIcon.hint')}</FormHelperText>
      </div>

      <Divider sx={{marginY: 2}} />
    </Stack>
  );
}

export default ButtonExtendedProperties;
