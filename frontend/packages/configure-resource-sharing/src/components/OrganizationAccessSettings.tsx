// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard, type QueryErrorMessageResolver} from '@thunderid/components';
import {useGetOrganizationUnitsByIds} from '@thunderid/configure-organization-units';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AccessChoiceCards, {type AccessChoice} from './AccessChoiceCards';
import GrantedOrganizationUnitList from './GrantedOrganizationUnitList';
import OrganizationUnitChipList from './OrganizationUnitChipList';
import OrganizationUnitSelectDialog, {type OrganizationUnitSelection} from './OrganizationUnitSelectDialog';
import SwitchAccessConfirmDialog from './SwitchAccessConfirmDialog';
import type {OrganizationAccessState} from '../hooks/useOrganizationAccess';
import type {
  AccessSwitch,
  ChildUnitsAccess,
  OrganizationAccessDraft,
  OrganizationAccessScope,
  OtherTreesAccess,
} from '../models/organization-access';
import {getBlockedNoneReason, type BlockedNoneReason} from '../utils/switchOrganizationAccess';
import translateOrganizationAccessWording, {
  organizationAccessWordingKey,
  type OrganizationAccessWordingKey,
} from '../utils/translateOrganizationAccessWording';

type OpenPicker = 'childUnits' | 'childUnitExclusions' | 'everyUnitExclusions' | 'trees' | 'everyTreeExclusions';

export interface OrganizationAccessSettingsProps {
  /**
   * The resource's Organization Access state, from {@link useOrganizationAccess}.
   */
  access: OrganizationAccessState;
  /**
   * The organization unit that owns the resource.
   */
  ownerOuId: string;
  /**
   * The resource's display name, used in headings and confirmations.
   */
  resourceName: string;
  /**
   * Makes everything read-only, as for a resource a resource file declares.
   */
  readOnly?: boolean;
  /**
   * Whether units can hold settings of their own for this resource, which switching choices clears.
   */
  hasUnitSettings?: boolean;
  /**
   * The i18n namespace holding the resource's error catalog, e.g. `applications` for `APP-` codes.
   */
  errorNamespace?: string;
  resolveErrorMessage?: QueryErrorMessageResolver;
}

const unique = (ids: string[]): string[] => [...new Set(ids)];

/**
 * The Organization Access screen of a shareable resource: which organization units of the owner's
 * tree can use it, and which other trees can. Every resource that mounts sharing endpoints renders
 * this with its own {@link OrganizationAccessState}.
 */
export default function OrganizationAccessSettings({
  access,
  ownerOuId,
  resourceName,
  readOnly = false,
  hasUnitSettings = false,
  errorNamespace = 'resourceSharing',
  resolveErrorMessage = undefined,
}: OrganizationAccessSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [openPicker, setOpenPicker] = useState<OpenPicker | null>(null);
  const [pendingSwitch, setPendingSwitch] = useState<AccessSwitch | null>(null);

  const {draft, offers, updateDraft, savedPolicy} = access;
  const locked = readOnly || access.isDeclared;
  const childNoneBlocked = getBlockedNoneReason(draft, 'childUnitsAccess', savedPolicy);
  const treesNoneBlocked = getBlockedNoneReason(draft, 'otherTreesAccess', savedPolicy);
  const blockedNoneDescription = (reason: BlockedNoneReason, otherSection: string): string =>
    reason === 'otherIsNone'
      ? t('resourceSharing:noneBlocked.otherIsNone', 'Not available while {{section}} is set to None.', {
          section: otherSection,
        })
      : t(
          'resourceSharing:noneBlocked.dropsEveryTree',
          'Not available. It also removes Every tree, which leaves nothing granted.',
        );

  const referencedIds = useMemo(
    () =>
      unique([
        ownerOuId,
        ...draft.childUnitExclusions,
        ...draft.childUnits.map((unit) => unit.ouId),
        ...draft.everyUnitExclusions,
        ...draft.trees,
        ...draft.everyTreeExclusions,
      ]),
    [ownerOuId, draft],
  );
  const unitsById = useGetOrganizationUnitsByIds(referencedIds);
  const ownerName = unitsById.get(ownerOuId)?.name ?? ownerOuId;
  // The sentences that depend on what the resource is come from its own wording namespace, if it has one.
  const wording = (key: OrganizationAccessWordingKey): string =>
    translateOrganizationAccessWording(t, access.wordingNamespace, key, {resource: resourceName, owner: ownerName});

  const scopeChoices: AccessChoice<OrganizationAccessScope>[] = [
    {
      value: 'owner',
      title: t('resourceSharing:scope.owner.title', '{{owner}} only', {owner: ownerName}),
      description: wording('ownerOnly'),
    },
    {
      value: 'chosen',
      title: t('resourceSharing:scope.chosen.title', '{{owner}} and chosen units', {owner: ownerName}),
      description: t(
        'resourceSharing:scope.chosen.description',
        'Choose child units of {{owner}} and other trees below.',
        {owner: ownerName},
      ),
    },
    {
      value: 'everyUnit',
      title: t('resourceSharing:scope.everyUnit.title', 'Every organization unit'),
      description: t(
        'resourceSharing:scope.everyUnit.description',
        'Every unit in every tree, now and later, with optional exceptions.',
      ),
    },
  ];
  // "Every organization unit" only comes from a resource file or an import. While the draft holds
  // it, it is the only choice shown, and changing the scope is how an admin leaves it.
  const visibleScopeChoices = scopeChoices.filter((choice) =>
    draft.scope === 'everyUnit' ? choice.value === 'everyUnit' : choice.value !== 'everyUnit',
  );
  const childChoices: AccessChoice<ChildUnitsAccess>[] = [
    {
      value: 'none',
      title: t('resourceSharing:childUnits.choices.none.title', 'None'),
      description: childNoneBlocked
        ? blockedNoneDescription(childNoneBlocked, t('resourceSharing:noneBlocked.otherTrees', 'other trees'))
        : t('resourceSharing:childUnits.choices.none.description', 'No child unit has access.'),
      disabled: childNoneBlocked !== null,
    },
    {
      value: 'all',
      title: t('resourceSharing:childUnits.choices.all.title', 'All child units'),
      description: t(
        'resourceSharing:childUnits.choices.all.description',
        'Every unit under {{owner}}, now and later.',
        {owner: ownerName},
      ),
    },
    {
      value: 'selected',
      title: t('resourceSharing:childUnits.choices.selected.title', 'Selected child units'),
      description: t(
        'resourceSharing:childUnits.choices.selected.description',
        'Pick units and how far below each one access reaches.',
      ),
    },
  ];
  const treeChoices: AccessChoice<OtherTreesAccess>[] = [
    {
      value: 'none',
      title: t('resourceSharing:trees.choices.none.title', 'None'),
      description: treesNoneBlocked
        ? blockedNoneDescription(treesNoneBlocked, t('resourceSharing:noneBlocked.childUnits', 'child units'))
        : t('resourceSharing:trees.choices.none.description', 'No other tree has access.'),
      disabled: treesNoneBlocked !== null,
    },
    {
      value: 'selected',
      title: t('resourceSharing:trees.choices.selected.title', 'Selected trees'),
      description: t('resourceSharing:trees.choices.selected.description', 'Grant top units one by one.'),
    },
    {
      value: 'every',
      title: t('resourceSharing:trees.choices.every.title', 'Every tree'),
      description: offers.everyTree
        ? t(
            'resourceSharing:trees.choices.every.description',
            'The top unit of every tree, now and later, with optional exceptions.',
          )
        : t(
            'resourceSharing:trees.choices.every.unavailable',
            "Every tree's top unit. Only set through a resource file or an import.",
          ),
      disabled: !offers.everyTree,
    },
  ];
  const choicesOf: Record<AccessSwitch['field'], AccessChoice<string>[]> = {
    scope: scopeChoices,
    childUnitsAccess: childChoices,
    otherTreesAccess: treeChoices,
  };
  const labelOf = (field: AccessSwitch['field'], value: string): string =>
    choicesOf[field].find((choice) => choice.value === value)?.title ?? '';

  const requestSwitch = (field: AccessSwitch['field'], to: AccessSwitch['to']): void => {
    if (locked) return;
    const accessSwitch = access.previewSwitch(field, to);
    if (accessSwitch.cleared.length === 0) {
      access.applySwitch(accessSwitch);
    } else {
      setPendingSwitch(accessSwitch);
    }
  };

  const edit = (update: (current: OrganizationAccessDraft) => Partial<OrganizationAccessDraft>): void => {
    updateDraft((current) => ({...current, ...update(current)}));
  };

  const addSelection = (selection: OrganizationUnitSelection[]): void => {
    const ids = selection.map((item) => item.ouId);
    switch (openPicker) {
      case 'childUnits':
        edit((current) => ({
          childUnits: [
            ...current.childUnits,
            ...selection
              .filter((item) => !current.childUnits.some((unit) => unit.ouId === item.ouId))
              .map((item) => ({ouId: item.ouId, reach: item.reach ?? 'subtree'})),
          ],
        }));
        break;
      case 'childUnitExclusions':
        edit((current) => ({childUnitExclusions: unique([...current.childUnitExclusions, ...ids])}));
        break;
      case 'everyUnitExclusions':
        edit((current) => ({everyUnitExclusions: unique([...current.everyUnitExclusions, ...ids])}));
        break;
      case 'trees':
        edit((current) => ({trees: unique([...current.trees, ...ids])}));
        break;
      case 'everyTreeExclusions':
        edit((current) => ({everyTreeExclusions: unique([...current.everyTreeExclusions, ...ids])}));
        break;
      default:
        break;
    }
    setOpenPicker(null);
  };

  const tForErrors = (key: string, options?: Record<string, unknown>): string =>
    t(key.includes(':') ? key : `${errorNamespace}:${key}`, options);

  if (access.isLoading) {
    return (
      <Box sx={{display: 'flex', justifyContent: 'center', py: 6}}>
        <CircularProgress />
      </Box>
    );
  }

  if (access.loadError) {
    return (
      <QueryErrorNotice
        error={access.loadError}
        t={tForErrors}
        variant="block"
        title={wording('loadFailedTitle')}
        fallbackKey={organizationAccessWordingKey(access.wordingNamespace, 'loadFailedMessage')}
        fallbackDefaultValue={wording('loadFailedMessage')}
        resolveErrorMessage={resolveErrorMessage}
        onRetry={access.reload}
      />
    );
  }

  const {scope} = draft;
  const childMode = draft.childUnitsAccess;
  const treesMode = draft.otherTreesAccess;
  // Selected with nothing picked yet grants nothing either, so it counts the same as None here.
  const nothingChosen =
    scope === 'chosen' &&
    (childMode === 'none' || (childMode === 'selected' && draft.childUnits.length === 0)) &&
    (treesMode === 'none' || (treesMode === 'selected' && draft.trees.length === 0));
  const dropsEveryUnit = offers.everyUnit && scope !== 'everyUnit';
  const dropsEveryTree = offers.everyTree && scope === 'chosen' && treesMode !== 'every';

  const exceptionsField = (
    ouIds: string[],
    hint: string,
    picker: 'childUnitExclusions' | 'everyUnitExclusions' | 'everyTreeExclusions',
  ): JSX.Element => (
    <Stack spacing={1}>
      <Typography variant="subtitle2">{t('resourceSharing:exceptions.title', 'Except')}</Typography>
      <Typography variant="caption" color="text.secondary">
        {hint}
      </Typography>
      <OrganizationUnitChipList
        ouIds={ouIds}
        unitsById={unitsById}
        emptyLabel={t('resourceSharing:exceptions.empty', 'No exceptions.')}
        addLabel={t('resourceSharing:exceptions.add', 'Add exception')}
        onAdd={locked ? undefined : () => setOpenPicker(picker)}
        onRemove={
          locked ? undefined : (ouId) => edit((current) => ({[picker]: current[picker].filter((id) => id !== ouId)}))
        }
      />
    </Stack>
  );

  const heading = wording('title');

  return (
    <Stack spacing={3}>
      <SettingsCard title={heading} description={wording('description')}>
        <Stack spacing={2.5}>
          <AccessChoiceCards
            label={heading}
            value={scope}
            choices={visibleScopeChoices}
            disabled={locked}
            onChange={(value) => requestSwitch('scope', value)}
          />

          {dropsEveryUnit && (
            <Alert severity="warning">
              {t(
                'resourceSharing:lost.everyUnit',
                'Saving deletes the "Every organization unit" policy. It can only be set in a resource file or by importing, so it can\'t be chosen again here.',
              )}
            </Alert>
          )}

          {nothingChosen && (
            <Alert severity="info">
              {t(
                'resourceSharing:nothingChosen',
                'Nothing is granted below yet. Saving like this is the same as "{{owner}} only".',
                {owner: ownerName},
              )}
            </Alert>
          )}

          {scope === 'everyUnit' && (
            <Stack spacing={2}>
              <Alert severity="info" icon={false}>
                {t(
                  'resourceSharing:everyUnit.summary',
                  'Every organization unit in every tree, at any depth. Units and trees created later get access automatically.',
                )}
              </Alert>
              {exceptionsField(
                draft.everyUnitExclusions,
                t(
                  'resourceSharing:exceptions.anyHint',
                  'Any unit in any tree. Excluding a unit also excludes everything below it.',
                ),
                'everyUnitExclusions',
              )}
              {!locked && (
                <Box
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    flexWrap: 'wrap',
                    gap: 1.5,
                    p: 2,
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '10px',
                  }}
                >
                  <Typography variant="body2" sx={{flex: '1 1 320px'}}>
                    <Box component="strong" sx={{fontWeight: 600}}>
                      {t('resourceSharing:changeScope.question', 'Need to grant only some units?')}
                    </Box>{' '}
                    {t(
                      'resourceSharing:changeScope.description',
                      'Changing the scope deletes the "Every organization unit" policy and its exceptions, then lets you choose child units and other trees. Once you save, it can\'t be set again here.',
                    )}
                  </Typography>
                  <Button variant="outlined" size="small" onClick={() => requestSwitch('scope', 'chosen')}>
                    {t('resourceSharing:changeScope.action', 'Change scope')}
                  </Button>
                </Box>
              )}
            </Stack>
          )}
        </Stack>
      </SettingsCard>

      {scope === 'chosen' && (
        <SettingsCard
          title={t('resourceSharing:childUnits.sectionTitle', 'Child organization units of {{owner}}', {
            owner: ownerName,
          })}
          description={t('resourceSharing:childUnits.sectionDescription', "Units in {{owner}}'s own hierarchy.", {
            owner: ownerName,
          })}
        >
          <Stack spacing={2.5}>
            <AccessChoiceCards
              label={t('resourceSharing:childUnits.sectionTitle', 'Child organization units of {{owner}}', {
                owner: ownerName,
              })}
              value={childMode}
              choices={childChoices}
              disabled={locked}
              onChange={(value) => requestSwitch('childUnitsAccess', value)}
            />

            {childMode === 'all' &&
              exceptionsField(
                draft.childUnitExclusions,
                t(
                  'resourceSharing:exceptions.childHint',
                  'Direct child units only. Excluding a unit also excludes everything below it.',
                ),
                'childUnitExclusions',
              )}

            {childMode === 'selected' && (
              <Stack spacing={1.5}>
                {!locked && (
                  <Box>
                    <Button
                      variant="contained"
                      size="small"
                      startIcon={<Plus size={14} />}
                      onClick={() => setOpenPicker('childUnits')}
                    >
                      {t('resourceSharing:childUnits.grant', 'Grant access')}
                    </Button>
                  </Box>
                )}
                <GrantedOrganizationUnitList
                  units={draft.childUnits}
                  unitsById={unitsById}
                  readOnly={locked}
                  emptyLabel={t(
                    'resourceSharing:childUnits.empty',
                    'No child units selected yet. Use Grant access to pick them.',
                  )}
                  onReachChange={(ouId, reach) =>
                    edit((current) => ({
                      childUnits: current.childUnits.map((unit) => (unit.ouId === ouId ? {...unit, reach} : unit)),
                    }))
                  }
                  onRemove={(ouId) =>
                    edit((current) => ({childUnits: current.childUnits.filter((unit) => unit.ouId !== ouId)}))
                  }
                />
              </Stack>
            )}
          </Stack>
        </SettingsCard>
      )}

      {scope === 'chosen' && (
        <SettingsCard
          title={t('resourceSharing:trees.title', 'Other organization trees')}
          description={t(
            'resourceSharing:trees.description',
            "Top units of trees outside {{owner}}'s. Units below a top unit don't get access unless it passes access on to them.",
            {owner: ownerName},
          )}
        >
          <Stack spacing={2.5}>
            <AccessChoiceCards
              label={t('resourceSharing:trees.title', 'Other organization trees')}
              value={treesMode}
              choices={treeChoices}
              disabled={locked}
              onChange={(value) => requestSwitch('otherTreesAccess', value)}
            />

            {dropsEveryTree && (
              <Alert severity="warning">
                {t(
                  'resourceSharing:lost.everyTree',
                  'Saving removes "Every tree". It can only be set in a resource file or by importing, so it can\'t be chosen again here.',
                )}
              </Alert>
            )}

            {treesMode === 'every' &&
              exceptionsField(
                draft.everyTreeExclusions,
                t('resourceSharing:exceptions.rootHint', 'Top units only.'),
                'everyTreeExclusions',
              )}

            {treesMode === 'selected' && (
              <Stack spacing={1.5}>
                {!locked && (
                  <Box>
                    <Button
                      variant="outlined"
                      size="small"
                      startIcon={<Plus size={14} />}
                      onClick={() => setOpenPicker('trees')}
                    >
                      {t('resourceSharing:trees.add', 'Add a tree')}
                    </Button>
                  </Box>
                )}
                <GrantedOrganizationUnitList
                  units={draft.trees.map((ouId) => ({ouId}))}
                  unitsById={unitsById}
                  readOnly={locked}
                  emptyLabel={t('resourceSharing:trees.empty', 'No trees picked yet. Use Add a tree to pick them.')}
                  fixedReachLabel={t('resourceSharing:trees.topUnitOnly', 'Top unit only')}
                  onRemove={(ouId) => edit((current) => ({trees: current.trees.filter((id) => id !== ouId)}))}
                />
              </Stack>
            )}
          </Stack>
        </SettingsCard>
      )}

      {openPicker === 'childUnits' && (
        <OrganizationUnitSelectDialog
          open
          title={t('resourceSharing:dialogs.childUnits.title', 'Grant access to child units')}
          description={t(
            'resourceSharing:dialogs.childUnits.description',
            'Only units directly under {{owner}} can be picked. Units deeper down are reached through their parent.',
            {owner: ownerName},
          )}
          confirmLabel={t('resourceSharing:childUnits.grant', 'Grant access')}
          source={{type: 'children', parentId: ownerOuId}}
          lockedIds={draft.childUnits.map((unit) => unit.ouId)}
          lockedLabel={t('resourceSharing:picker.alreadyGranted', 'Already granted')}
          withReach
          onClose={() => setOpenPicker(null)}
          onConfirm={addSelection}
        />
      )}
      {openPicker === 'trees' && (
        <OrganizationUnitSelectDialog
          open
          title={t('resourceSharing:dialogs.trees.title', 'Grant access to other trees')}
          description={t(
            'resourceSharing:dialogs.trees.description',
            'Pick the top unit of each tree. Units below it get access only if it passes access on.',
          )}
          confirmLabel={t('resourceSharing:childUnits.grant', 'Grant access')}
          source={{type: 'roots'}}
          hiddenIds={[ownerOuId]}
          lockedIds={draft.trees}
          lockedLabel={t('resourceSharing:picker.alreadyGranted', 'Already granted')}
          onClose={() => setOpenPicker(null)}
          onConfirm={addSelection}
        />
      )}
      {(openPicker === 'childUnitExclusions' ||
        openPicker === 'everyUnitExclusions' ||
        openPicker === 'everyTreeExclusions') && (
        <OrganizationUnitSelectDialog
          open
          tone="exclude"
          title={t('resourceSharing:dialogs.exceptions.title', 'Add exceptions')}
          description={
            {
              childUnitExclusions: t(
                'resourceSharing:dialogs.exceptions.childDescription',
                "Choose child units of {{owner}} that shouldn't get access. Everything below an excluded unit is excluded too.",
                {owner: ownerName},
              ),
              everyUnitExclusions: t(
                'resourceSharing:dialogs.exceptions.anyDescription',
                "Choose any unit, at any depth, that shouldn't get access. Everything below it is excluded too.",
              ),
              everyTreeExclusions: t(
                'resourceSharing:dialogs.exceptions.rootDescription',
                "Choose top units that shouldn't get access.",
              ),
            }[openPicker]
          }
          confirmLabel={t('resourceSharing:dialogs.exceptions.confirm', 'Apply exceptions')}
          source={
            {
              childUnitExclusions: {type: 'children', parentId: ownerOuId} as const,
              everyUnitExclusions: {type: 'allTrees'} as const,
              everyTreeExclusions: {type: 'roots'} as const,
            }[openPicker]
          }
          hiddenIds={openPicker === 'everyTreeExclusions' ? [ownerOuId] : []}
          lockedIds={draft[openPicker]}
          lockedLabel={t('resourceSharing:picker.alreadyExcluded', 'Already excluded')}
          onClose={() => setOpenPicker(null)}
          onConfirm={addSelection}
        />
      )}

      <SwitchAccessConfirmDialog
        accessSwitch={pendingSwitch}
        fromLabel={pendingSwitch ? labelOf(pendingSwitch.field, draft[pendingSwitch.field]) : ''}
        toLabel={pendingSwitch ? labelOf(pendingSwitch.field, pendingSwitch.to) : ''}
        ownerName={ownerName}
        resourceName={resourceName}
        unitsById={unitsById}
        hasUnitSettings={hasUnitSettings}
        wordingNamespace={access.wordingNamespace}
        onCancel={() => setPendingSwitch(null)}
        onConfirm={(accessSwitch) => {
          access.applySwitch(accessSwitch);
          setPendingSwitch(null);
        }}
      />
    </Stack>
  );
}
