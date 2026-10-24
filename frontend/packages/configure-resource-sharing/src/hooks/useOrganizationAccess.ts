// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useMemo, useState} from 'react';
import useGetSharingPolicies from '../api/useGetSharingPolicies';
import useSaveSharingPolicy from '../api/useSaveSharingPolicy';
import type {
  AccessSwitch,
  OrganizationAccessDraft,
  OrganizationAccessOffers,
  SharingPolicySavePlan,
} from '../models/organization-access';
import type {SharingPolicy} from '../models/sharing-policy';
import draftToSharingSections from '../utils/draftToSharingSections';
import getOrganizationAccessOffers from '../utils/getOrganizationAccessOffers';
import planSharingPolicySave from '../utils/planSharingPolicySave';
import sharingPolicyToDraft from '../utils/sharingPolicyToDraft';
import switchOrganizationAccess from '../utils/switchOrganizationAccess';

export interface UseOrganizationAccessOptions {
  /**
   * The resource's path relative to the server URL, e.g. `applications/<id>`.
   */
  resourcePath: string;
  /**
   * Whether to load the resource's sharing policies.
   */
  enabled?: boolean;
}

/**
 * The Organization Access state of one shareable resource, held by the page that owns the
 * resource's save action so that it survives switching tabs.
 */
export interface OrganizationAccessState {
  isLoading: boolean;
  /**
   * The failure to load the resource's sharing policies, if any.
   */
  loadError: Error | null;
  reload: () => void;
  /**
   * The owner-issued policy as last read, or `undefined` when the resource is not shared.
   */
  savedPolicy: SharingPolicy | undefined;
  /**
   * The draft being edited, or the saved access when nothing has been changed.
   */
  draft: OrganizationAccessDraft;
  offers: OrganizationAccessOffers;
  /**
   * Whether a resource file declares the policy, which makes it read-only here.
   */
  isDeclared: boolean;
  hasChanges: boolean;
  /**
   * Changes the draft without switching a choice.
   */
  updateDraft: (update: (draft: OrganizationAccessDraft) => OrganizationAccessDraft) => void;
  /**
   * Works out what switching a choice would clear, without applying it.
   */
  previewSwitch: (field: AccessSwitch['field'], to: AccessSwitch['to']) => AccessSwitch;
  applySwitch: (accessSwitch: AccessSwitch) => void;
  /**
   * Discards the draft and reloads the saved access.
   */
  reset: () => void;
  /**
   * Saves the draft. Resolves when there is nothing to save, rejects with the request's error.
   */
  save: () => Promise<void>;
  isSaving: boolean;
  saveError: Error | null;
}

/**
 * Holds the Organization Access screen's state for one shareable resource: it reads the resource's
 * owner-issued sharing policy, keeps the admin's draft, and saves the draft with the fewest requests
 * the sharing API allows.
 *
 * @param options - The resource to manage
 * @returns The resource's Organization Access state
 *
 * @example
 * ```tsx
 * const access = useOrganizationAccess({resourcePath: `applications/${applicationId}`});
 * return <OrganizationAccessSettings access={access} ... />;
 * ```
 *
 * @public
 */
export default function useOrganizationAccess({
  resourcePath,
  enabled = true,
}: UseOrganizationAccessOptions): OrganizationAccessState {
  const policiesQuery = useGetSharingPolicies(resourcePath, enabled);
  const saveMutation = useSaveSharingPolicy(resourcePath);
  const [editedDraft, setEditedDraft] = useState<OrganizationAccessDraft | null>(null);

  const savedPolicy = useMemo(
    () => policiesQuery.data?.find((policy) => policy.stage === 'share'),
    [policiesQuery.data],
  );
  const savedDraft = useMemo(() => sharingPolicyToDraft(savedPolicy), [savedPolicy]);
  const draft = editedDraft ?? savedDraft;
  const offers = useMemo(() => getOrganizationAccessOffers(savedPolicy), [savedPolicy]);

  const plan = useMemo<SharingPolicySavePlan>(
    () => (editedDraft ? planSharingPolicySave(savedPolicy, draftToSharingSections(editedDraft)) : {type: 'none'}),
    [editedDraft, savedPolicy],
  );

  const {reset: resetSaveMutation, mutateAsync: runSave} = saveMutation;
  const {refetch} = policiesQuery;

  const updateDraft = useCallback(
    (update: (current: OrganizationAccessDraft) => OrganizationAccessDraft): void => {
      // A previous save error is stale the moment the draft changes again.
      resetSaveMutation();
      setEditedDraft((current) => update(current ?? savedDraft));
    },
    [resetSaveMutation, savedDraft],
  );

  const previewSwitch = useCallback(
    (field: AccessSwitch['field'], to: AccessSwitch['to']): AccessSwitch =>
      switchOrganizationAccess(draft, field, to, savedPolicy),
    [draft, savedPolicy],
  );

  const applySwitch = useCallback(
    (accessSwitch: AccessSwitch): void => updateDraft(() => accessSwitch.next),
    [updateDraft],
  );

  const reset = useCallback((): void => {
    resetSaveMutation();
    setEditedDraft(null);
    // refetch() runs even while the query is disabled, so a resource that doesn't load its
    // policies would otherwise request them on every page reset.
    if (enabled && resourcePath) void refetch();
  }, [resetSaveMutation, refetch, enabled, resourcePath]);

  const save = useCallback(async (): Promise<void> => {
    if (plan.type === 'none') return;
    await runSave(plan);
    await refetch();
    setEditedDraft(null);
  }, [plan, runSave, refetch]);

  return {
    isLoading: policiesQuery.isLoading,
    loadError: policiesQuery.error,
    reload: () => void refetch(),
    savedPolicy,
    draft,
    offers,
    isDeclared: savedPolicy?.isReadOnly === true,
    hasChanges: plan.type !== 'none',
    updateDraft,
    previewSwitch,
    applySwitch,
    reset,
    save,
    isSaving: saveMutation.isPending,
    saveError: saveMutation.error,
  };
}
