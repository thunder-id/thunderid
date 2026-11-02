// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {AccessSwitch, ClearedAccessItem} from '../models/organization-access';
import translateOrganizationAccessWording, {
  type OrganizationAccessWordingKey,
} from '../utils/translateOrganizationAccessWording';

export interface SwitchAccessConfirmDialogProps {
  accessSwitch: AccessSwitch | null;
  /**
   * Display name of the choice being left.
   */
  fromLabel: string;
  /**
   * Display name of the choice being switched to.
   */
  toLabel: string;
  ownerName: string;
  resourceName: string;
  unitsById: Map<string, OrganizationUnit>;
  /**
   * Whether units can hold settings of their own for this resource, which a switch also clears.
   */
  hasUnitSettings?: boolean;
  /**
   * The i18n namespace holding the resource's own wording for stopping sharing, e.g. `users`.
   */
  wordingNamespace?: string;
  onCancel: () => void;
  onConfirm: (accessSwitch: AccessSwitch) => void;
}

/**
 * Asks before switching an Organization Access choice, listing everything the switch clears. The
 * switch only changes the draft: nothing is removed until the resource is saved.
 */
export default function SwitchAccessConfirmDialog({
  accessSwitch,
  fromLabel,
  toLabel,
  ownerName,
  resourceName,
  unitsById,
  hasUnitSettings = false,
  wordingNamespace = undefined,
  onCancel,
  onConfirm,
}: SwitchAccessConfirmDialogProps): JSX.Element {
  const {t} = useTranslation();
  const stopsSharing = accessSwitch?.field === 'scope' && accessSwitch.to === 'owner';
  const wording = (key: OrganizationAccessWordingKey): string =>
    translateOrganizationAccessWording(t, wordingNamespace, key, {resource: resourceName, owner: ownerName});
  // Leaving "Every organization unit" deletes that policy, so the dialog says so in its own words.
  const changesScope = accessSwitch?.field === 'scope' && accessSwitch.losesEveryUnit;

  const names = (ouIds: string[]): string => ouIds.map((ouId) => unitsById.get(ouId)?.name ?? ouId).join(', ');

  const describe = (item: ClearedAccessItem): string => {
    switch (item.kind) {
      case 'childUnitExclusions':
        return t('resourceSharing:switch.cleared.childUnitExclusions', 'Child unit exceptions: {{names}}', {
          names: names(item.ouIds),
        });
      case 'childUnits':
        return t('resourceSharing:switch.cleared.childUnits', 'Granted child units: {{names}}', {
          names: names(item.ouIds),
        });
      case 'trees':
        return t('resourceSharing:switch.cleared.trees', 'Granted trees: {{names}}', {
          names: names(item.ouIds),
        });
      case 'everyUnit':
        return item.ouIds.length > 0
          ? t(
              'resourceSharing:switch.cleared.everyUnitWithExceptions',
              'The "Every organization unit" policy, with its exceptions: {{names}}',
              {names: names(item.ouIds)},
            )
          : t('resourceSharing:switch.cleared.everyUnit', 'The "Every organization unit" policy');
      case 'everyTree':
        return item.ouIds.length > 0
          ? t('resourceSharing:switch.cleared.everyTreeWithExceptions', 'Every tree, with its exceptions: {{names}}', {
              names: names(item.ouIds),
            })
          : t('resourceSharing:switch.cleared.everyTree', 'Every tree');
      default:
        return '';
    }
  };

  return (
    <Dialog open={Boolean(accessSwitch)} onClose={onCancel} maxWidth="sm" fullWidth>
      <DialogTitle>
        {stopsSharing && wording('stopSharingTitle')}
        {!stopsSharing && changesScope && t('resourceSharing:switch.titleChangeScope', 'Change the scope?')}
        {!stopsSharing &&
          !changesScope &&
          t('resourceSharing:switch.title', 'Switch to {{choice}}?', {choice: toLabel})}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2}>
          <DialogContentText>
            {stopsSharing && wording('stopSharingIntro')}
            {!stopsSharing &&
              changesScope &&
              t(
                'resourceSharing:switch.introChangeScope',
                'This deletes the "Every organization unit" policy, so {{resource}} stops reaching every organization unit. Access then starts as All child units of {{owner}}, which you can change below. Deleting the policy clears:',
                {owner: ownerName, resource: resourceName},
              )}
            {!stopsSharing &&
              !changesScope &&
              t(
                'resourceSharing:switch.intro',
                "What you set up for {{choice}} doesn't carry over. Switching clears:",
                {
                  choice: fromLabel,
                },
              )}
          </DialogContentText>
          <Box component="ul" sx={{m: 0, pl: 3, py: 1.5, pr: 2, bgcolor: 'action.hover', borderRadius: '10px'}}>
            {accessSwitch?.cleared.map((item) => (
              <Typography component="li" variant="body2" key={item.kind} sx={{py: 0.25}}>
                {describe(item)}
              </Typography>
            ))}
          </Box>
          {accessSwitch?.losesEveryUnit && (
            <Alert severity="warning">
              {t(
                'resourceSharing:switch.cantRestore.everyUnit',
                '"Every organization unit" can only be set in a resource file or by importing. Once you save, it can\'t be chosen again here.',
              )}
            </Alert>
          )}
          {accessSwitch?.losesEveryTree && (
            <Alert severity="warning">
              {t(
                'resourceSharing:switch.cantRestore.everyTree',
                '"Every tree" can only be set in a resource file or by importing. Once you save, it can\'t be chosen again here.',
              )}
            </Alert>
          )}
          {hasUnitSettings && (
            <Alert severity="info">
              {t(
                'resourceSharing:switch.unitSettings',
                'Unit settings are cleared too. Units lose the settings they chose for themselves within their terms.',
              )}
            </Alert>
          )}
          <Typography variant="body2" color="text.secondary">
            {t(
              'resourceSharing:switch.notYet',
              'Nothing is removed until you save. Until then, Reset brings everything back.',
            )}
          </Typography>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel}>{t('resourceSharing:switch.keep', 'Keep current choice')}</Button>
        <Button
          variant="contained"
          color="error"
          onClick={() => {
            if (accessSwitch) onConfirm(accessSwitch);
          }}
        >
          {stopsSharing && wording('stopSharingConfirm')}
          {!stopsSharing &&
            changesScope &&
            t('resourceSharing:switch.confirmChangeScope', 'Delete policy and change scope')}
          {!stopsSharing && !changesScope && t('resourceSharing:switch.confirm', 'Switch and clear')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
