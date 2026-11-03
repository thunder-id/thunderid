// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DiscardChangesDialog, DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateAuthorizationDetailType from '../api/useCreateAuthorizationDetailType';
import FieldsSection from '../components/authorization-detail-types/FieldsSection';
import GeneralSection from '../components/authorization-detail-types/GeneralSection';
import ProcessorSection from '../components/authorization-detail-types/ProcessorSection';
import useAuthorizationDetailTypeForm, {
  errorLogDetails,
  SAVE_ERROR_SECTION,
  type AuthorizationDetailTypeFormSection,
} from '../hooks/useAuthorizationDetailTypeForm';
import useAuthorizationDetailTypeRoutes from '../hooks/useAuthorizationDetailTypeRoutes';
import useDiscardConfirmation from '../hooks/useDiscardConfirmation';
import useProblemFocus from '../hooks/useProblemFocus';

const AuthorizationDetailTypeCreateStep = {
  DETAILS: 'DETAILS',
  FIELDS: 'FIELDS',
  PROCESSOR: 'PROCESSOR',
} as const;

type AuthorizationDetailTypeCreateStep = keyof typeof AuthorizationDetailTypeCreateStep;

const STEP_ORDER: AuthorizationDetailTypeCreateStep[] = [
  AuthorizationDetailTypeCreateStep.DETAILS,
  AuthorizationDetailTypeCreateStep.FIELDS,
  AuthorizationDetailTypeCreateStep.PROCESSOR,
];

/** The part of the form each step edits, which must be valid to continue. */
const STEP_SECTION: Record<AuthorizationDetailTypeCreateStep, AuthorizationDetailTypeFormSection> = {
  DETAILS: 'general',
  FIELDS: 'fields',
  PROCESSOR: 'processor',
};

/** The id of the content of the current step. */
const STEP_CONTENT_ID = 'authorization-detail-type-step';

export default function CreateAuthorizationDetailTypePage(): JSX.Element {
  const navigate = useNavigate();
  const routes = useAuthorizationDetailTypeRoutes();
  const {t} = useTranslation();
  const logger = useLogger('CreateAuthorizationDetailTypePage');
  const createType = useCreateAuthorizationDetailType();
  const form = useAuthorizationDetailTypeForm();
  const focusProblem = useProblemFocus();
  // A second click before the create starts would send the type again: the state that disables the
  // button reaches it only after the next render.
  const submittingRef = useRef(false);

  const [currentStep, setCurrentStep] = useState<AuthorizationDetailTypeCreateStep>(
    AuthorizationDetailTypeCreateStep.DETAILS,
  );

  const steps: Record<AuthorizationDetailTypeCreateStep, {label: string; title: string; description: string}> = useMemo(
    () => ({
      DETAILS: {
        label: t('authorizationDetailTypes:create.steps.details', 'Details'),
        title: t('authorizationDetailTypes:create.details.title', "Let's collect some details about your intent"),
        description: t(
          'authorizationDetailTypes:general.description',
          'How clients refer to this intent, how it is named in the Console, and whether users approve each request.',
        ),
      },
      FIELDS: {
        label: t('authorizationDetailTypes:create.steps.fields', 'Schema'),
        title: t('authorizationDetailTypes:create.fields.title', 'What does the user approve?'),
        description: t(
          'authorizationDetailTypes:schema.description',
          'The values a request carries, on the consent screen the user approves them on. Select the summary or a field on the card to edit it.',
        ),
      },
      PROCESSOR: {
        label: t('authorizationDetailTypes:create.steps.processor', 'External processor'),
        title: t('authorizationDetailTypes:create.processor.title', 'Should your system check each request?'),
        description: t(
          'authorizationDetailTypes:processor.description',
          'Your system that approves or refuses each request and can fill in values. Required for client credentials requests.',
        ),
      },
    }),
    [t],
  );

  const currentIndex = STEP_ORDER.indexOf(currentStep);
  const isFirstStep = currentIndex === 0;
  const isLastStep = currentIndex === STEP_ORDER.length - 1;

  // The wizard is spent once the create succeeds: the navigation away still has to run, and re-enabling
  // in between would let the user create a duplicate.
  const isSubmitting = createType.isPending || createType.isSuccess;

  // Every step but the last needs only its own part to be valid; creating needs the whole form.
  const stepValid = form.sectionValid[STEP_SECTION[currentStep]] && (!isLastStep || form.isValid);

  // Closing asks first once something is entered.
  const discard = useDiscardConfirmation(form.hasChanges);

  const handleClose = (): void => {
    discard.confirm(() => {
      (async (): Promise<void> => {
        await navigate(routes.list());
      })().catch((err: unknown) => {
        logger.error('Failed to navigate to intents list', {error: err});
      });
    });
  };

  const handleNext = (): void => {
    if (isSubmitting || submittingRef.current) return;

    if (!stepValid) {
      // Going on shows the problems of the step, and the first one takes focus. The fields select
      // their first problem themselves.
      form.revealIssues();
      if (currentStep !== AuthorizationDetailTypeCreateStep.FIELDS) {
        focusProblem({containerId: STEP_CONTENT_ID, target: null});
      }
      return;
    }

    if (!isLastStep) {
      setCurrentStep(STEP_ORDER[currentIndex + 1]);
      return;
    }

    submittingRef.current = true;
    createType.mutate(form.buildRequest(), {
      onSuccess: (created) => {
        // The wizard is left behind, so going back does not open it again, empty.
        (async (): Promise<void> => {
          await navigate(routes.detail(created.id), {replace: true});
        })().catch((err: unknown) => {
          logger.error('Failed to navigate after create', {error: err});
        });
      },
      onError: (err: Error) => {
        submittingRef.current = false;
        logger.error('Failed to create intent', errorLogDetails(err));
        // An error about one value opens the step that edits it, where it is shown beside the value
        // and takes focus. The fields select the field an error is about themselves.
        const target = form.reportSaveError(err);
        const step = target && STEP_ORDER.find((candidate) => STEP_SECTION[candidate] === SAVE_ERROR_SECTION[target]);
        if (target && step) {
          setCurrentStep(step);
          if (step !== AuthorizationDetailTypeCreateStep.FIELDS) focusProblem({containerId: STEP_CONTENT_ID, target});
        }
      },
    });
  };

  const handleBack = (): void => {
    if (!isFirstStep) setCurrentStep(STEP_ORDER[currentIndex - 1]);
  };

  const renderStep = (): JSX.Element => {
    switch (currentStep) {
      case AuthorizationDetailTypeCreateStep.FIELDS:
        return <FieldsSection form={form} deferProcessorIssues />;
      case AuthorizationDetailTypeCreateStep.PROCESSOR:
        return <ProcessorSection form={form} />;
      default:
        return <GeneralSection form={form} />;
    }
  };

  return (
    <>
      <DocumentTitle title={t('authorizationDetailTypes:listing.addIntent', 'Add intent')} />
      <FullScreenCreationWizardLayout
        onClose={handleClose}
        progress={((currentIndex + 1) / STEP_ORDER.length) * 100}
        breadcrumbItems={STEP_ORDER.slice(0, currentIndex + 1).map((step, index, array) => ({
          key: step,
          label: steps[step].label,
          onClick: index < array.length - 1 && !isSubmitting ? () => setCurrentStep(step) : undefined,
        }))}
        // The schema composer needs more horizontal room than the single-column forms.
        contentMaxWidth={currentStep === AuthorizationDetailTypeCreateStep.FIELDS ? 1400 : 800}
        footer={
          <Box sx={{display: 'flex', justifyContent: isFirstStep ? 'flex-end' : 'space-between', gap: 2}}>
            {!isFirstStep && (
              <Button variant="outlined" onClick={handleBack} sx={{minWidth: 100}} disabled={isSubmitting}>
                {t('common:actions.back', 'Back')}
              </Button>
            )}

            <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
              {isSubmitting && <CircularProgress size={20} />}
              {/* Kept enabled while the step has problems, so going on can show them. */}
              <Button variant="contained" disabled={isSubmitting} sx={{minWidth: 100}} onClick={handleNext}>
                {isLastStep
                  ? isSubmitting
                    ? t('authorizationDetailTypes:create.creating', 'Creating…')
                    : t('authorizationDetailTypes:create.submit', 'Create intent')
                  : t('common:actions.continue', 'Continue')}
              </Button>
            </Box>
          </Box>
        }
      >
        {form.saveError && !form.saveErrorTarget && (
          <Alert
            severity="error"
            sx={{mb: 4}}
            onClose={() => {
              form.clearSaveError();
              createType.reset();
            }}
          >
            {form.saveError}
          </Alert>
        )}

        <Stack direction="column" spacing={4}>
          <Box>
            <Typography variant="h1" gutterBottom>
              {steps[currentStep].title}
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {steps[currentStep].description}
            </Typography>
          </Box>

          <Box id={STEP_CONTENT_ID} tabIndex={-1} sx={{outline: 'none'}}>
            {renderStep()}
          </Box>
        </Stack>
      </FullScreenCreationWizardLayout>

      <DiscardChangesDialog
        open={discard.open}
        onClose={discard.onClose}
        onConfirm={discard.onConfirm}
        description={t(
          'authorizationDetailTypes:discard.description',
          'You have changes to this intent that are not saved. If you leave now, they are lost.',
        )}
      />
    </>
  );
}
