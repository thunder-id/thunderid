/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {cx} from '@emotion/css';
import {CreateOrganizationPayload, createPackageComponentLogger, Preferences} from '@thunderid/browser';
import {ChangeEvent, CSSProperties, FC, FormEvent, ReactElement, ReactNode, useState} from 'react';
import useStyles from './BaseCreateOrganization.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import useTranslation from '../../../hooks/useTranslation';
import AlertPrimitive from '../../primitives/Alert/Alert';
import Button from '../../primitives/Button/Button';
import DialogPrimitive from '../../primitives/Dialog/Dialog';
import FormControl from '../../primitives/FormControl/FormControl';
import InputLabel from '../../primitives/InputLabel/InputLabel';
import TextField from '../../primitives/TextField/TextField';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'BaseCreateOrganization',
);

/**
 * Interface for organization form data.
 */
export interface OrganizationFormData {
  description: string;
  handle: string;
  name: string;
}

/**
 * Props interface for the BaseCreateOrganization component.
 */
export interface BaseCreateOrganizationProps {
  cardLayout?: boolean;
  className?: string;
  defaultParentId?: string;
  error?: string | null;
  initialValues?: Partial<OrganizationFormData>;
  loading?: boolean;
  mode?: 'inline' | 'popup';
  onCancel?: () => void;
  onOpenChange?: (open: boolean) => void;
  onSubmit?: (payload: CreateOrganizationPayload) => void | Promise<void>;
  onSuccess?: (organization: any) => void;
  open?: boolean;
  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;
  renderAdditionalFields?: () => ReactNode;
  style?: CSSProperties;

  title?: string;
}

/**
 * Removes special characters except space and hyphen from the organization name
 * and generates a valid handle.
 * @param name
 * @returns
 */
const generateHandleFromName = (name: string): string =>
  name
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');

/**
 * BaseCreateOrganization component provides the core functionality for creating organizations.
 * This component serves as the base for framework-specific implementations.
 */
export const BaseCreateOrganization: FC<BaseCreateOrganizationProps> = ({
  cardLayout = true,
  className = '',
  defaultParentId = '',
  error,
  initialValues = {},
  loading = false,
  mode = 'inline',
  onCancel,
  onOpenChange,
  onSubmit,
  onSuccess,
  open = false,
  preferences,
  renderAdditionalFields,
  style,
  title = 'Create Organization',
}: BaseCreateOrganizationProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: ReturnType<typeof useStyles> = useStyles(theme, colorScheme);
  const {t} = useTranslation(preferences?.i18n);
  const [formData, setFormData] = useState<OrganizationFormData>({
    description: '',
    handle: '',
    name: '',
    ...initialValues,
  });
  const [formErrors, setFormErrors] = useState<Partial<OrganizationFormData> & {avatar?: string}>({});

  const validateForm = (): boolean => {
    const errors: Partial<OrganizationFormData> = {};

    if (!formData.name.trim()) {
      errors.name = 'Organization name is required';
    }

    if (!formData.handle.trim()) {
      errors.handle = 'Organization handle is required';
    } else if (!/^[a-z0-9-]+$/.test(formData.handle)) {
      errors.handle = 'Handle can only contain lowercase letters, numbers, and hyphens';
    }

    if (!formData.description.trim()) {
      errors.description = 'Organization description is required';
    }

    setFormErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleInputChange = (field: keyof OrganizationFormData, value: string): void => {
    setFormData((prev: OrganizationFormData) => ({
      ...prev,
      [field]: value,
    }));

    if (formErrors[field]) {
      setFormErrors((prev: Partial<OrganizationFormData> & {avatar?: string}) => ({
        ...prev,
        [field]: undefined,
      }));
    }
  };

  /**
   * Handles changes to the organization name input.
   * Automatically generates the organization handle based on the name if the handle is not set or matches
   *
   * @param value - The new value for the organization name.
   */
  const handleNameChange = (value: string): void => {
    handleInputChange('name', value);

    if (!formData.handle || formData.handle === generateHandleFromName(formData.name)) {
      const newHandle: string = generateHandleFromName(value);
      handleInputChange('handle', newHandle);
    }
  };

  const handleSubmit = async (e: FormEvent): Promise<void> => {
    e.preventDefault();

    if (!validateForm() || loading) {
      return;
    }

    const payload: CreateOrganizationPayload = {
      description: formData.description.trim(),
      name: formData.name.trim(),
      orgHandle: formData.handle.trim(),
      parentId: defaultParentId,
      type: 'TENANT',
    };

    try {
      await onSubmit?.(payload);
      if (onSuccess) {
        onSuccess(payload);
      }
    } catch (submitError) {
      // Error handling is done by parent component
      logger.error('Form submission error:');
    }
  };

  const createOrganizationContent: ReactElement = (
    <div className={cx(styles['root'], cardLayout && styles['card'], className)} style={style}>
      <div className={cx(styles['content'])}>
        <form id="create-organization-form" className={cx(styles['form'])} onSubmit={handleSubmit}>
          {error && (
            <AlertPrimitive variant="error" className={styles['errorAlert']}>
              <AlertPrimitive.Title>Error</AlertPrimitive.Title>
              <AlertPrimitive.Description>{error}</AlertPrimitive.Description>
            </AlertPrimitive>
          )}
          <div className={cx(styles['fieldGroup'])}>
            <TextField
              label={`${t('elements.fields.organization.name.label')}`}
              placeholder={t('elements.fields.organization.name.placeholder')}
              value={formData.name}
              onChange={(e: ChangeEvent<HTMLInputElement>): void => handleNameChange(e.target.value)}
              disabled={loading}
              required
              error={formErrors.name}
              className={cx(styles['input'])}
            />
          </div>
          <div className={cx(styles['fieldGroup'])}>
            <TextField
              label={`${t('elements.fields.organization.handle.label') || 'Organization Handle'}`}
              placeholder={t('elements.fields.organization.handle.placeholder') || 'my-organization'}
              value={formData.handle}
              onChange={(e: ChangeEvent<HTMLInputElement>): void => handleInputChange('handle', e.target.value)}
              disabled={loading}
              required
              error={formErrors.handle}
              helperText="This will be your organization's unique identifier. Only lowercase letters, numbers, and hyphens are allowed."
              className={cx(styles['input'])}
            />
          </div>
          <div className={cx(styles['fieldGroup'])}>
            <FormControl error={formErrors.description}>
              <InputLabel required>{t('elements.fields.organization.description.label')}</InputLabel>
              <textarea
                className={cx(styles['textarea'], formErrors.description && styles['textareaError'])}
                placeholder={t('organization.create.description.placeholder')}
                value={formData.description}
                onChange={(e: ChangeEvent<HTMLTextAreaElement>): void =>
                  handleInputChange('description', e.target.value)
                }
                disabled={loading}
                required
              />
            </FormControl>
          </div>
          {renderAdditionalFields && renderAdditionalFields()}
        </form>
        <div className={cx(styles['actions'])}>
          {onCancel && (
            <Button type="button" variant="outline" onClick={onCancel} disabled={loading}>
              {t('organization.create.buttons.cancel.text')}
            </Button>
          )}
          <Button type="submit" variant="solid" color="primary" disabled={loading} form="create-organization-form">
            {loading
              ? t('organization.create.buttons.create_organization.loading.text')
              : t('organization.create.buttons.create_organization.text')}
          </Button>
        </div>
      </div>
    </div>
  );

  if (mode === 'popup') {
    return (
      <DialogPrimitive open={open} onOpenChange={onOpenChange}>
        <DialogPrimitive.Content>
          <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
          <div className={styles['popup']}>{createOrganizationContent}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive>
    );
  }

  return createOrganizationContent;
};

export default BaseCreateOrganization;
