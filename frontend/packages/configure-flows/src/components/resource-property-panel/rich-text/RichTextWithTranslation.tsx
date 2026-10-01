// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, FormHelperText, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {SquareFunction} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import type {ToolbarPluginProps} from './helper-plugins/ToolbarPlugin';
import RichText from './RichText';
import useResourceFieldError from '../../../hooks/useResourceFieldError';
import type {Resource} from '../../../models/resources';
import DynamicValuePopover from '../DynamicValuePopover';

/**
 * Props interface for the RichTextWithTranslation component.
 */
export interface RichTextWithTranslationProps {
  /**
   * Options to customize the rich text editor toolbar.
   */
  ToolbarProps?: ToolbarPluginProps;
  /**
   * Listener for changes in the rich text editor content.
   *
   * @param value - The HTML string representation of the rich text editor content.
   */
  onChange: (value: string) => void;
  /**
   * Additional CSS class names to apply to the rich text editor container.
   */
  className?: string;
  /**
   * The resource associated with the rich text editor.
   */
  resource: Resource;
}

/**
 * Rich text editor component with translation support.
 */
function RichTextWithTranslation({
  ToolbarProps = {},
  className = '',
  onChange,
  resource,
}: RichTextWithTranslationProps): ReactElement {
  const {t} = useTranslation();
  const [isDynamicValuePopoverOpen, setIsDynamicValuePopoverOpen] = useState<boolean>(false);
  const [buttonEl, setButtonEl] = useState<HTMLButtonElement | null>(null);

  /**
   * Get the error message for the rich text field.
   */
  const errorMessage: string = useResourceFieldError(resource?.id, 'text');

  return (
    <Box sx={{position: 'relative'}}>
      <RichText ToolbarProps={ToolbarProps} className={className} onChange={onChange} resource={resource} />
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
      <Tooltip title={t('flows:core.elements.textPropertyField.tooltip.configureDynamicValue')}>
        <IconButton
          ref={setButtonEl}
          onClick={() => setIsDynamicValuePopoverOpen(!isDynamicValuePopoverOpen)}
          size="small"
          sx={{position: 'absolute', top: 8, right: 8}}
        >
          <SquareFunction size={13} />
        </IconButton>
      </Tooltip>
      <DynamicValuePopover
        open={isDynamicValuePopoverOpen}
        anchorEl={buttonEl}
        propertyKey="richText"
        onClose={() => setIsDynamicValuePopoverOpen(false)}
        value={String((resource as Resource & {label?: string})?.label ?? '')}
        onChange={(newValue: string) => onChange(newValue)}
      />
    </Box>
  );
}

export default RichTextWithTranslation;
