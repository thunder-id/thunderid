// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Typography, useTheme} from '@wso2/oxygen-ui';
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import {motion} from 'framer-motion';
import type {JSX, ReactNode} from 'react';
import OrganizationUnitTreePicker from './OrganizationUnitTreePicker';

const MotionBox = motion.create(Box);

export interface OrganizationUnitPickerScreenProps {
  /**
   * Icon rendered in the gradient square above the title.
   */
  icon: ReactNode;

  /**
   * Screen title, e.g. "Where should this application belong?".
   */
  title: string;

  /**
   * Screen subtitle, shown under the title.
   */
  subtitle: string;

  /**
   * The selected organization unit ID.
   */
  value: string;

  /**
   * When set, scopes the tree to this organization unit's subtree instead of the global root.
   */
  rootOuId?: string;

  /**
   * Callback invoked when the user picks a different organization unit.
   */
  onChange: (ouId: string) => void;

  /**
   * Callback invoked when the back link is clicked.
   */
  onBack: () => void;

  /**
   * Callback invoked when Continue is clicked. Only fires once an organization unit is selected.
   */
  onContinue: () => void;

  /**
   * Label for the back link.
   */
  backLabel: string;

  /**
   * Label for the continue button.
   */
  continueLabel: string;

  /**
   * Why the selected organization unit cannot be used, shown under the tree. While it is set,
   * Continue stays disabled.
   */
  error?: string;

  /**
   * Whether the first organization unit is selected when nothing is. Turn it off when the first one
   * is often unusable, so the screen does not open on an error.
   */
  autoSelectFirst?: boolean;
}

/**
 * Full-screen, wizard-chrome-free step for picking the organization unit a new resource (an
 * application, a role, a group, ...) will belong to. Meant to be shown once, before the
 * resource-specific creation wizard mounts, so the rest of that wizard can assume an organization
 * unit is already chosen. Reuses {@link OrganizationUnitTreePicker} so nested organization units
 * stay browsable, framed in a larger panel to suit a dedicated full-screen step.
 *
 * @public
 */
export default function OrganizationUnitPickerScreen({
  icon,
  title,
  subtitle,
  value,
  rootOuId = undefined,
  onChange,
  onBack,
  onContinue,
  backLabel,
  continueLabel,
  error = undefined,
  autoSelectFirst = true,
}: OrganizationUnitPickerScreenProps): JSX.Element {
  const theme = useTheme();

  return (
    <Box
      sx={{
        minHeight: '100vh',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        position: 'relative',
        px: 3,
        py: 8,
      }}
    >
      <Box
        onClick={onBack}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            onBack();
          }
        }}
        sx={{
          position: 'absolute',
          top: 24,
          left: 28,
          cursor: 'pointer',
          color: 'text.secondary',
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
          fontSize: 13,
          '&:hover': {color: 'text.primary'},
        }}
      >
        <ChevronLeft size={16} />
        {backLabel}
      </Box>

      <Box sx={{width: '100%', maxWidth: 640, textAlign: 'center'}}>
        <MotionBox initial={{opacity: 0, y: 14}} animate={{opacity: 1, y: 0}} transition={{duration: 0.5}}>
          <Box
            sx={{
              width: 56,
              height: 56,
              borderRadius: '16px',
              background: theme.gradient?.primary,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              mx: 'auto',
              mb: 2.75,
              color: theme.palette.primary.contrastText,
            }}
          >
            {icon}
          </Box>
          <Typography variant="h4" gutterBottom>
            {title}
          </Typography>
          <Typography variant="body1" color="text.secondary" sx={{mb: 4}}>
            {subtitle}
          </Typography>
        </MotionBox>

        <MotionBox
          initial={{opacity: 0, y: 22, scale: 0.98}}
          animate={{opacity: 1, y: 0, scale: 1}}
          transition={{duration: 0.5, delay: 0.15}}
          sx={{textAlign: 'left'}}
        >
          <OrganizationUnitTreePicker
            id="organization-unit-picker-screen-tree"
            value={value}
            rootOuId={rootOuId}
            onChange={onChange}
            maxHeight={420}
            spacious
            autoSelectFirst={autoSelectFirst}
          />
          {error && (
            <Alert severity="error" sx={{mt: 3}}>
              {error}
            </Alert>
          )}
        </MotionBox>

        <MotionBox
          initial={{opacity: 0, y: 14}}
          animate={{opacity: 1, y: 0}}
          transition={{duration: 0.6, delay: 0.1}}
          sx={{display: 'flex', justifyContent: 'center', mt: 4.25}}
        >
          <Button variant="contained" onClick={onContinue} disabled={!value || Boolean(error)} sx={{minWidth: 220}}>
            {continueLabel}
          </Button>
        </MotionBox>
      </Box>
    </Box>
  );
}
