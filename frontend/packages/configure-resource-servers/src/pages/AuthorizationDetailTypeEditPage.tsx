// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  DiscardChangesDialog,
  DocumentTitle,
  PageLoadingAnimation,
  QueryErrorNotice,
  SettingsCard,
  UnsavedChangesBar,
} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
  Chip,
  IconButton,
  PageContent,
  PageTitle,
  Stack,
  Tab,
  Tabs,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowLeft, CircleAlert, Edit} from '@wso2/oxygen-ui-icons-react';
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type JSX,
  type MouseEvent,
  type ReactNode,
  type SyntheticEvent,
} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetAuthorizationDetailType from '../api/useGetAuthorizationDetailType';
import useUpdateAuthorizationDetailType from '../api/useUpdateAuthorizationDetailType';
import AskEveryTimeSetting from '../components/authorization-detail-types/AskEveryTimeSetting';
import AuthorizationDetailTypeDeleteDialog from '../components/authorization-detail-types/AuthorizationDetailTypeDeleteDialog';
import FieldsSection from '../components/authorization-detail-types/FieldsSection';
import ProcessorSection from '../components/authorization-detail-types/ProcessorSection';
import QuickCopySection from '../components/authorization-detail-types/QuickCopySection';
import useAuthorizationDetailTypeForm, {
  errorLogDetails,
  SAVE_ERROR_SECTION,
  saveErrorCode,
  type AuthorizationDetailTypeFormSection,
} from '../hooks/useAuthorizationDetailTypeForm';
import useAuthorizationDetailTypeRoutes from '../hooks/useAuthorizationDetailTypeRoutes';
import useDiscardConfirmation from '../hooks/useDiscardConfirmation';
import useProblemFocus from '../hooks/useProblemFocus';
import type {AuthorizationDetailType} from '../models/authorization-detail-type';

interface TabPanelProps {
  children?: ReactNode;
  index: number;
  value: number;
}

/** The id of the panel of a tab. */
function tabPanelId(index: number): string {
  return `authorization-detail-type-tabpanel-${index}`;
}

/** The id of the notice that the type changed or was deleted elsewhere. */
const REMOTE_NOTICE_ID = 'authorization-detail-type-remote-notice';

/** The id of the heading, which takes focus once a save mounts the form again. */
const HEADING_ID = 'authorization-detail-type-heading';

/** The id of the header, where the name is edited. */
const HEADER_ID = 'authorization-detail-type-header';

function TabPanel({children = undefined, value, index}: TabPanelProps): JSX.Element {
  return (
    <Box
      role="tabpanel"
      hidden={value !== index}
      id={tabPanelId(index)}
      aria-labelledby={`authorization-detail-type-tab-${index}`}
      tabIndex={-1}
      sx={{pt: 3, outline: 'none'}}
    >
      {value === index && children}
    </Box>
  );
}

/** The tabs, in order, each editing one part of the form. The Advanced tab follows them. */
const TAB_SECTIONS: AuthorizationDetailTypeFormSection[] = ['general', 'fields', 'processor'];

const TAB_ADVANCED = TAB_SECTIONS.length;

/** What happened to the type elsewhere since the form started from it. */
type RemoteState = 'changed' | 'deleted' | null;

/** Whether a load found no type with the id, as for a type deleted elsewhere. */
function isNotFound(error: Error): boolean {
  return saveErrorCode(error) === 'RAR-1002' || (error as {response?: {status?: number}}).response?.status === 404;
}

/** The parts of a type an administrator edits, to tell whether it changed elsewhere. */
function editedParts(detailType: AuthorizationDetailType): string {
  return JSON.stringify([
    detailType.name,
    detailType.description ?? '',
    detailType.version,
    detailType.schema,
    detailType.consentTemplate ?? '',
    detailType.reuse,
    detailType.processor?.endpoint ?? '',
    detailType.processor?.authentication ?? null,
  ]);
}

interface AuthorizationDetailTypeEditorProps {
  detailType: AuthorizationDetailType;
  activeTab: number;
  onTabChange: (tab: number) => void;
  /** Called with the saved type, so the form starts again from it. */
  onSaved: (saved: AuthorizationDetailType) => void;
  /**
   * Called to discard the changes, so the form starts again from the latest type, and to take a change
   * made elsewhere while the form has none.
   */
  onReset: () => void;
  /** What happened to the type elsewhere since the form started from it. */
  remote: RemoteState;
  /** The error of the last refresh, other than the type being gone, while the form stays open. */
  refreshError: Error | null;
  /** Refreshes the type. */
  onRetry: () => void;
  /** Resolves an error through the intent error catalog. */
  tForErrors: (key: string, options?: Record<string, unknown>) => string;
  /** Fetches the latest type, and resolves to whether it is still the one the form started from. */
  checkLatest: () => Promise<boolean>;
  /** Whether the heading takes focus when the form mounts, as after a save. */
  focusHeading: boolean;
}

/**
 * The tabs and the save bar of one authorization details type. The form starts from the type it is
 * given, so the page mounts it again after a save or a reset.
 */
function AuthorizationDetailTypeEditor({
  detailType,
  activeTab,
  onTabChange,
  onSaved,
  onReset,
  remote,
  refreshError,
  onRetry,
  tForErrors,
  checkLatest,
  focusHeading,
}: AuthorizationDetailTypeEditorProps): JSX.Element {
  const navigate = useNavigate();
  const routes = useAuthorizationDetailTypeRoutes();
  const {t} = useTranslation();
  const logger = useLogger('AuthorizationDetailTypeEditPage');
  const updateType = useUpdateAuthorizationDetailType();
  const form = useAuthorizationDetailTypeForm(detailType);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  // The name and the description are edited in the header. Each holds the value its edit started from, so
  // Escape puts it back, or null while it is not edited.
  const [nameBeforeEdit, setNameBeforeEdit] = useState<string | null>(null);
  const [descriptionBeforeEdit, setDescriptionBeforeEdit] = useState<string | null>(null);
  const discard = useDiscardConfirmation(form.hasChanges);
  const focusProblem = useProblemFocus();
  // A click while a save is in flight is ignored, as it would send the type again: the state that
  // disables the button reaches it only after the next render.
  const savingRef = useRef(false);
  const [saving, setSaving] = useState(false);
  // Counts the saves stopped by a change made elsewhere, so the notice that tells of it is brought into
  // view once it shows.
  const [stoppedSaves, setStoppedSaves] = useState(0);
  const shownStoppedSaves = useRef(0);

  // A type changed elsewhere replaces an unedited form at once, as loading the latest version does; the
  // change is reported only when it would replace edits.
  const changedElsewhere = remote === 'changed';
  useEffect(() => {
    if (changedElsewhere && !form.hasChanges) onReset();
  }, [changedElsewhere, form.hasChanges, onReset]);

  const remoteNoticeShown = remote === 'deleted' || (changedElsewhere && form.hasChanges);
  useEffect(() => {
    if (!remoteNoticeShown || stoppedSaves === shownStoppedSaves.current) return;
    shownStoppedSaves.current = stoppedSaves;
    const notice = document.getElementById(REMOTE_NOTICE_ID);
    notice?.scrollIntoView({block: 'nearest'});
    notice?.focus({preventScroll: true});
  }, [remoteNoticeShown, stoppedSaves]);

  useEffect(() => {
    if (focusHeading) document.getElementById(HEADING_ID)?.focus();
  }, [focusHeading]);

  const listUrl = routes.list();
  const readOnly = Boolean(detailType.isReadOnly);

  const tabLabels: Record<AuthorizationDetailTypeFormSection, string> = {
    general: t('authorizationDetailTypes:edit.tab.general', 'General'),
    fields: t('authorizationDetailTypes:edit.tab.fields', 'Schema'),
    processor: t('authorizationDetailTypes:edit.tab.processor', 'External processor'),
  };

  const nameServerError = form.saveErrorTarget === 'name' ? form.saveError : null;
  let nameHelper: string | undefined;
  if (nameServerError) nameHelper = nameServerError;
  else if (form.nameError)
    nameHelper = t('authorizationDetailTypes:form.nameError', 'Enter a name of up to 100 characters.');
  else if (form.nameMissing) nameHelper = t('authorizationDetailTypes:form.nameRequired', 'Enter a name.');
  const nameEditorOpen = !readOnly && nameBeforeEdit !== null;
  const descriptionEditorOpen = !readOnly && descriptionBeforeEdit !== null;

  const finishNameEdit = (cancel: boolean): void => {
    if (nameBeforeEdit === null) return;
    // A name with a problem stays in its field, with the problem under it.
    if (!cancel && (form.nameError || nameServerError)) return;
    // An empty name is not applied, so the name goes back to what it was.
    if (cancel || !form.name.trim()) form.setName(nameBeforeEdit);
    else form.setName(form.name.trim());
    setNameBeforeEdit(null);
  };

  const finishDescriptionEdit = (cancel: boolean): void => {
    form.setDescription(cancel ? (descriptionBeforeEdit ?? '') : form.description.trim());
    setDescriptionBeforeEdit(null);
  };

  // The name is edited in the header and the type identifier of a saved type cannot change, so the General
  // tab has a problem only when its type identifier has one.
  const sectionValid = (section: AuthorizationDetailTypeFormSection): boolean =>
    section === 'general' ? !form.typeError && !form.typeMissing : form.sectionValid[section];

  // A problem may sit on a tab that is not open, so the save bar names the tabs that have one, and
  // each of those tabs is marked.
  const errorSection =
    form.saveErrorTarget && form.saveErrorTarget !== 'name' ? SAVE_ERROR_SECTION[form.saveErrorTarget] : null;
  const tabHasProblem = (section: AuthorizationDetailTypeFormSection): boolean =>
    (form.hasChanges && !sectionValid(section)) || errorSection === section;
  // A tab with a value a failed save is about is named too, as the value shows its error only there.
  const invalidTabs = TAB_SECTIONS.filter((section) => !sectionValid(section) || errorSection === section).map(
    (section) => tabLabels[section],
  );

  const goToList = (): void => {
    (async (): Promise<void> => {
      await navigate(listUrl);
    })().catch((err: unknown) => {
      logger.error('Failed to navigate back', {error: err});
    });
  };

  const handleSave = (): void => {
    if (readOnly || savingRef.current) return;
    if (!form.isValid) {
      // Saving shows every problem, from the name in the header or the first tab that has one. The fields
      // select their first problem themselves.
      form.revealIssues();
      if (form.nameError || form.nameMissing) {
        focusProblem({containerId: HEADER_ID, target: 'name'});
        return;
      }
      const first = TAB_SECTIONS.findIndex((section) => !sectionValid(section));
      if (first >= 0) {
        onTabChange(first);
        if (TAB_SECTIONS[first] !== 'fields') focusProblem({containerId: tabPanelId(first), target: null});
      }
      return;
    }

    savingRef.current = true;
    setSaving(true);
    const finish = (): void => {
      savingRef.current = false;
      setSaving(false);
    };
    const data = form.buildRequest();
    (async (): Promise<void> => {
      // A save does not carry the version the form started from, so it would replace a change made
      // elsewhere since the form opened without a word. The latest type is fetched first, and a change
      // or a deletion stops the save, so the notice that tells of it shows. A save made with the notice
      // shown replaces the change, as the notice says.
      if (!changedElsewhere && !(await checkLatest())) {
        finish();
        setStoppedSaves((count) => count + 1);
        return;
      }
      updateType.mutate(
        {id: detailType.id, data},
        {
          onSuccess: (saved) => {
            finish();
            onSaved(saved);
          },
          onError: (err: Error) => {
            finish();
            logger.error('Failed to save intent', errorLogDetails(err));
            // The type changed while the save was applied, so the latest version is fetched, and the
            // notice that it changed elsewhere offers to load it.
            if (saveErrorCode(err) === 'RAR-1016') onRetry();
            // An error about one value opens the tab that edits it, where it is shown beside the value
            // and takes focus. The fields select the field an error is about themselves. The name is edited
            // in the header, which shows its error.
            const target = form.reportSaveError(err);
            if (target === 'name') {
              setNameBeforeEdit(detailType.name);
              focusProblem({containerId: HEADER_ID, target});
            } else if (target) {
              const tab = TAB_SECTIONS.indexOf(SAVE_ERROR_SECTION[target]);
              onTabChange(tab);
              if (SAVE_ERROR_SECTION[target] !== 'fields') focusProblem({containerId: tabPanelId(tab), target});
            }
          },
        },
      );
    })().catch((err: unknown) => {
      finish();
      logger.error('Failed to check the intent before saving', errorLogDetails(err as Error));
    });
  };

  return (
    <PageContent>
      <DocumentTitle title={detailType.name} />
      {readOnly && (
        <Alert severity="info" sx={{mb: 2}}>
          {t('common:messages.readOnlyResource', 'This resource is read-only and cannot be modified.')}
        </Alert>
      )}

      {refreshError && (
        <Box sx={{mb: 2}}>
          <QueryErrorNotice error={refreshError} t={tForErrors} variant="inline" onRetry={onRetry} />
        </Box>
      )}

      {remote === 'deleted' && (
        <Alert
          id={REMOTE_NOTICE_ID}
          tabIndex={-1}
          severity="error"
          sx={{mb: 2}}
          action={
            <Button color="inherit" size="small" onClick={goToList}>
              {t('authorizationDetailTypes:edit.back', 'Back to intents')}
            </Button>
          }
        >
          {form.hasChanges
            ? t(
                'authorizationDetailTypes:edit.deletedElsewhere',
                'This intent was deleted, so your changes cannot be saved.',
              )
            : t(
                'authorizationDetailTypes:edit.deletedElsewhereClean',
                'This intent was deleted, so it can no longer be edited.',
              )}
        </Alert>
      )}
      {changedElsewhere && form.hasChanges && (
        <Alert
          id={REMOTE_NOTICE_ID}
          tabIndex={-1}
          severity="warning"
          sx={{mb: 2}}
          action={
            <Button color="inherit" size="small" onClick={onReset} sx={{whiteSpace: 'nowrap'}}>
              {t('authorizationDetailTypes:edit.loadLatest', 'Load latest')}
            </Button>
          }
        >
          {t(
            'authorizationDetailTypes:edit.changedElsewhere',
            'Someone else changed this intent after you opened it. Saving replaces their changes. Loading the latest version shows them and discards your changes.',
          )}
        </Alert>
      )}

      <PageTitle>
        <PageTitle.BackButton
          component={
            <Link
              to={listUrl}
              onClick={(e: MouseEvent<HTMLAnchorElement>) => {
                if (!form.hasChanges) return;
                e.preventDefault();
                discard.confirm(goToList);
              }}
            />
          }
        >
          {t('authorizationDetailTypes:edit.back', 'Back to intents')}
        </PageTitle.BackButton>
        <PageTitle.Header component="div" id={HEADER_ID}>
          <Stack direction="row" alignItems="center" spacing={1} mb={1} data-save-error-target="name">
            {nameEditorOpen ? (
              <TextField
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                value={form.name}
                disabled={saving}
                onChange={(e) => form.setName(e.target.value)}
                onBlur={() => finishNameEdit(false)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') finishNameEdit(false);
                  else if (e.key === 'Escape') finishNameEdit(true);
                }}
                size="small"
                error={nameHelper !== undefined}
                helperText={nameHelper}
                slotProps={{htmlInput: {'aria-label': t('authorizationDetailTypes:form.name', 'Name')}}}
              />
            ) : (
              <>
                <Typography variant="h3" component="h1" id={HEADING_ID} tabIndex={-1} sx={{outline: 'none'}}>
                  {form.name}
                </Typography>
                {!readOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('authorizationDetailTypes:edit.editName', 'Edit intent name')}
                    disabled={saving}
                    onClick={() => setNameBeforeEdit(form.name)}
                    sx={{opacity: 0.6, '&:hover': {opacity: 1}}}
                  >
                    <Edit size={16} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader component="div">
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {descriptionEditorOpen ? (
              <TextField
                // eslint-disable-next-line jsx-a11y/no-autofocus
                autoFocus
                fullWidth
                multiline
                rows={2}
                value={form.description}
                disabled={saving}
                onChange={(e) => form.setDescription(e.target.value)}
                onBlur={() => finishDescriptionEdit(false)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) finishDescriptionEdit(false);
                  else if (e.key === 'Escape') finishDescriptionEdit(true);
                }}
                size="small"
                placeholder={t('authorizationDetailTypes:edit.descriptionPlaceholder', 'Add a description')}
                slotProps={{htmlInput: {'aria-label': t('authorizationDetailTypes:form.description', 'Description')}}}
                sx={{maxWidth: '600px', '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography component="span" variant="body2" color="text.secondary">
                  {form.description.trim() || t('authorizationDetailTypes:edit.noDescription', 'No description')}
                </Typography>
                {!readOnly && (
                  <IconButton
                    size="small"
                    aria-label={t('authorizationDetailTypes:edit.editDescription', 'Edit intent description')}
                    disabled={saving}
                    onClick={() => setDescriptionBeforeEdit(form.description)}
                    sx={{opacity: 0.6, '&:hover': {opacity: 1}, mt: -0.5}}
                  >
                    <Edit size={14} />
                  </IconButton>
                )}
              </>
            )}
          </Stack>
          <Box sx={{mt: 1, display: 'flex', flexWrap: 'wrap', gap: 1, alignItems: 'center'}}>
            <Typography
              component="span"
              variant="body2"
              color="text.secondary"
              sx={{fontFamily: 'monospace', fontSize: '0.8rem', wordBreak: 'break-all'}}
            >
              {detailType.type}
            </Typography>
            {/* The chip takes focus, so its explanation shows to keyboard users too. */}
            <Tooltip
              describeChild
              title={t(
                'authorizationDetailTypes:form.versionHint',
                "Increases when the fields, the consent summary or the approval reuse setting change, or when an external processor is added or removed. Other changes, such as the processor's endpoint or credentials, keep the version.",
              )}
            >
              <Chip
                label={t('authorizationDetailTypes:edit.version', 'Version {{version}}', {version: detailType.version})}
                size="small"
                variant="outlined"
                tabIndex={0}
                sx={{fontSize: '0.7rem'}}
              />
            </Tooltip>
          </Box>
        </PageTitle.SubHeader>
      </PageTitle>

      {form.definitionChanged && (
        <Alert severity="warning" sx={{mb: 2}}>
          {t(
            'authorizationDetailTypes:edit.newVersionNotice',
            'Saving creates a new version of this intent. Earlier approvals no longer apply.',
          )}
        </Alert>
      )}

      {/* Values entered while a save is in flight would be lost when the form starts again from the saved
          type, so the form takes no input until the save ends. */}
      <Box inert={saving}>
        <Tabs
          value={activeTab}
          onChange={(_e: SyntheticEvent, next: number) => onTabChange(next)}
          aria-label={t('authorizationDetailTypes:edit.tabs', 'Intent settings')}
        >
          {TAB_SECTIONS.map((section, index) => (
            <Tab
              key={section}
              label={
                tabHasProblem(section) ? (
                  <Stack component="span" direction="row" alignItems="center" spacing={0.75}>
                    <span>{tabLabels[section]}</span>
                    <Box
                      component="span"
                      role="img"
                      aria-label={t('authorizationDetailTypes:edit.tabHasProblems', 'Has problems')}
                      sx={{display: 'flex', color: 'error.main'}}
                    >
                      <CircleAlert size={14} />
                    </Box>
                  </Stack>
                ) : (
                  tabLabels[section]
                )
              }
              id={`authorization-detail-type-tab-${index}`}
              aria-controls={tabPanelId(index)}
              sx={{textTransform: 'none'}}
            />
          ))}
          {/* A read-only type has nothing to show on the Advanced tab. */}
          {!readOnly && (
            <Tab
              label={t('authorizationDetailTypes:edit.tab.advanced', 'Advanced')}
              id={`authorization-detail-type-tab-${TAB_ADVANCED}`}
              aria-controls={tabPanelId(TAB_ADVANCED)}
              sx={{textTransform: 'none'}}
            />
          )}
        </Tabs>

        <TabPanel value={activeTab} index={0}>
          <Stack spacing={3}>
            <QuickCopySection
              id={detailType.id}
              type={detailType.type}
              typeError={form.saveErrorTarget === 'type' ? form.saveError : null}
            />
            <SettingsCard
              title={t('authorizationDetailTypes:consent.title', 'Consent')}
              description={t(
                'authorizationDetailTypes:consent.description',
                'Whether users approve each request or an earlier approval can be reused.',
              )}
            >
              <AskEveryTimeSetting form={form} readOnly={readOnly} />
            </SettingsCard>
          </Stack>
        </TabPanel>

        <TabPanel value={activeTab} index={1}>
          <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
            {readOnly
              ? t(
                  'authorizationDetailTypes:schema.descriptionReadOnly',
                  'The values a request carries, on the consent screen the user approves them on. Select the summary or a field on the card to see it.',
                )
              : t(
                  'authorizationDetailTypes:schema.description',
                  'The values a request carries, on the consent screen the user approves them on. Select the summary or a field on the card to edit it.',
                )}
          </Typography>
          <FieldsSection form={form} readOnly={readOnly} />
        </TabPanel>

        <TabPanel value={activeTab} index={2}>
          <SettingsCard
            title={t('authorizationDetailTypes:processor.title', 'External processor')}
            description={t(
              'authorizationDetailTypes:processor.description',
              'Your system that approves or refuses each request and can fill in values. Required for client credentials requests.',
            )}
          >
            <ProcessorSection form={form} readOnly={readOnly} />
          </SettingsCard>
        </TabPanel>

        {!readOnly && (
          <TabPanel value={activeTab} index={TAB_ADVANCED}>
            <SettingsCard
              title={t('authorizationDetailTypes:edit.dangerZone.title', 'Danger zone')}
              description={t(
                'authorizationDetailTypes:edit.dangerZone.description',
                'Irreversible actions for this intent.',
              )}
            >
              <Typography variant="h6" gutterBottom color="error">
                {t('authorizationDetailTypes:edit.dangerZone.delete.title', 'Delete intent')}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
                {t(
                  'authorizationDetailTypes:edit.dangerZone.delete.description',
                  'Permanently delete this intent. Clients can no longer request it. This action cannot be undone.',
                )}
              </Typography>
              <Button variant="contained" color="error" onClick={() => setDeleteDialogOpen(true)}>
                {t('authorizationDetailTypes:edit.dangerZone.delete.button', 'Delete intent')}
              </Button>
            </SettingsCard>
          </TabPanel>
        )}
      </Box>

      <AuthorizationDetailTypeDeleteDialog
        detailType={deleteDialogOpen ? detailType : null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={goToList}
      />

      <DiscardChangesDialog
        open={discard.open}
        onClose={discard.onClose}
        onConfirm={discard.onConfirm}
        description={t(
          'authorizationDetailTypes:discard.description',
          'You have changes to this intent that are not saved. If you leave now, they are lost.',
        )}
      />

      {form.hasChanges && (
        <UnsavedChangesBar
          message={t('authorizationDetailTypes:edit.unsavedChanges', 'You have unsaved changes.')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('common:save', 'Save')}
          savingLabel={t('common:saving', 'Saving…')}
          isSaving={saving}
          saveDisabled={readOnly || remote === 'deleted'}
          error={
            // An error about one value is shown beside the value, so the bar shows only the others.
            (form.saveErrorTarget ? null : form.saveError) ??
            (invalidTabs.length > 0
              ? t('authorizationDetailTypes:edit.invalid', 'Fix the problems on these tabs before saving: {{tabs}}', {
                  tabs: invalidTabs.join(', '),
                })
              : undefined)
          }
          onReset={onReset}
          onSave={handleSave}
        />
      )}
    </PageContent>
  );
}

export default function AuthorizationDetailTypeEditPage(): JSX.Element {
  const {id} = useParams<{id: string}>();
  const navigate = useNavigate();
  const routes = useAuthorizationDetailTypeRoutes();
  const {t} = useTranslation();
  const logger = useLogger('AuthorizationDetailTypeEditPage');

  const {data, isLoading, error, refetch} = useGetAuthorizationDetailType(id ?? '');

  const [activeTab, setActiveTab] = useState(0);
  // The form starts from the type it is mounted with, so it is mounted again to discard the changes and,
  // after a save, from the type the server returned.
  const [formKey, setFormKey] = useState(0);
  const [base, setBase] = useState<AuthorizationDetailType | null>(null);
  const [saved, setSaved] = useState(false);
  if (data && base?.id !== data.id) setBase(data);

  // Resolves an error through the `authorizationDetailTypes` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `authorizationDetailTypes:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `authorizationDetailTypes:${key}`, options),
    [t],
  );

  const backButton = (
    <Button
      startIcon={<ArrowLeft size={16} />}
      onClick={() => {
        (async (): Promise<void> => {
          await navigate(routes.list());
        })().catch((err: unknown) => {
          logger.error('Failed to navigate back', {error: err});
        });
      }}
    >
      {t('authorizationDetailTypes:edit.back', 'Back to intents')}
    </Button>
  );

  const resourceTypeTitle = t('authorizationDetailTypes:listing.title', 'Intents');

  if (isLoading && !base) {
    return (
      <>
        <DocumentTitle title={resourceTypeTitle} />
        <PageLoadingAnimation />
      </>
    );
  }

  // A failed refresh keeps the open form, so the changes in it are not lost, and says so above it. A type
  // that does not exist is reported as not found, below.
  if (!base && error && !isNotFound(error)) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('authorizationDetailTypes:edit.errorTitle', 'Failed to load intent')}
          onRetry={() => void refetch()}
          action={backButton}
        />
      </PageContent>
    );
  }

  if (!base) {
    return (
      <PageContent>
        <DocumentTitle title={resourceTypeTitle} />
        <Alert severity="warning" sx={{mb: 2}}>
          {t('authorizationDetailTypes:edit.notFound', 'Intent not found.')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  // The type may change or go away elsewhere while it is open: a refresh tells.
  let remote: RemoteState = null;
  if (error && isNotFound(error)) remote = 'deleted';
  else if (data && editedParts(data) !== editedParts(base)) remote = 'changed';

  return (
    <AuthorizationDetailTypeEditor
      key={`${base.id}-${formKey}`}
      detailType={base}
      activeTab={activeTab}
      onTabChange={setActiveTab}
      remote={remote}
      refreshError={error && remote !== 'deleted' ? error : null}
      onRetry={() => void refetch()}
      tForErrors={tForErrors}
      checkLatest={async () => {
        const latest = await refetch();
        if (latest?.error && isNotFound(latest.error)) return false;
        // A type that cannot be fetched is saved anyway, and the save reports what is wrong.
        return !latest?.data || editedParts(latest.data) === editedParts(base);
      }}
      focusHeading={saved}
      onSaved={(savedType) => {
        setBase(savedType);
        setSaved(true);
        setFormKey((current) => current + 1);
      }}
      onReset={() => {
        if (data) setBase(data);
        setSaved(false);
        setFormKey((current) => current + 1);
      }}
    />
  );
}
