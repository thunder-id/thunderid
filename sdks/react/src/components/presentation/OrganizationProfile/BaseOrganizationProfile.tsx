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
import {OrganizationDetails, formatDate, Preferences} from '@thunderid/browser';
import {FC, ReactElement, ReactNode, useState, useCallback} from 'react';
import useStyles from './BaseOrganizationProfile.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import {Avatar} from '../../primitives/Avatar/Avatar';
import Button from '../../primitives/Button/Button';
import CardPrimitive from '../../primitives/Card/Card';
import DialogPrimitive from '../../primitives/Dialog/Dialog';
import KeyValueInput from '../../primitives/KeyValueInput/KeyValueInput';
import TextField from '../../primitives/TextField/TextField';

export interface BaseOrganizationProfileProps {
  /**
   * Callback fired when the cancel button is clicked (only used in editable mode).
   */
  cancelButtonText?: string;

  /**
   * Whether to display the profile in a card layout.
   */
  cardLayout?: boolean;

  /**
   * CSS class name for styling the component.
   */
  className?: string;

  /**
   * Whether the organization profile is editable.
   */
  editable?: boolean;

  /**
   * Component to render when no organization data is available.
   */
  fallback?: ReactElement;

  /**
   * Array of field configurations to display. Each field specifies what organization data to show.
   */
  fields?: {
    editable?: boolean;
    key: keyof OrganizationDetails | 'attributes';
    label: string;
    render?: (value: any, organization: OrganizationDetails) => ReactNode;
  }[];

  /**
   * Display mode for the component.
   */
  mode?: 'inline' | 'popup';

  /**
   * Callback fired when a field value changes.
   */
  onChange?: (field: string, value: any) => void;

  /**
   * Callback fired when the popup should be closed (only used in popup mode).
   */
  onOpenChange?: (open: boolean) => void;

  /**
   * Callback fired when the form is submitted (only used in editable mode).
   */
  onSubmit?: (data: any) => void;

  /**
   * Callback fired when the organization should be updated.
   */
  onUpdate?: (payload: any) => Promise<void>;

  /**
   * Whether the popup is open (only used in popup mode).
   */
  open?: boolean;

  /**
   * The organization details to display.
   */
  organization?: OrganizationDetails | null;

  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;

  /**
   * Text for the save button (only used in editable mode).
   */
  saveButtonText?: string;

  /**
   * Custom title for the profile.
   */
  title?: string;
}

/**
 * BaseOrganizationProfile component displays organization information in a
 * structured and styled format. It shows organization details such as name,
 * description, status, and other available information with support for inline editing.
 *
 * This is the base component that can be used in any context where you have
 * an organization object available. It provides editing capabilities similar to
 * the UserProfile component, allowing users to modify organization fields directly.
 *
 * @example
 * ```tsx
 * // Basic usage
 * <BaseOrganizationProfile organization={organizationData} />
 *
 * // With editing enabled and update handler
 * <BaseOrganizationProfile
 *   organization={organizationData}
 *   editable={true}
 *   onUpdate={async (payload) => {
 *     await updateOrganizationAPI(orgId, payload);
 *   }}
 * />
 *
 * // With card layout and custom title
 * <BaseOrganizationProfile
 *   organization={organizationData}
 *   cardLayout={true}
 *   title="Organization Details"
 *   fallback={<div>No organization data available</div>}
 * />
 *
 * // With custom fields configuration
 * <BaseOrganizationProfile
 *   organization={organizationData}
 *   fields={[
 *     { key: 'id', label: 'Organization ID', editable: false },
 *     { key: 'name', label: 'Organization Name', editable: true },
 *     { key: 'description', label: 'Description', editable: true, render: (value) => value || 'No description' },
 *     { key: 'created', label: 'Created Date', editable: false, render: (value) => new Date(value).toLocaleDateString() },
 *     { key: 'attributes', label: 'Custom Attributes', editable: true }
 *   ]}
 *   onUpdate={handleUpdate}
 * />
 *
 * // In popup mode
 * <BaseOrganizationProfile
 *   organization={organizationData}
 *   mode="popup"
 *   open={isOpen}
 *   onOpenChange={setIsOpen}
 *   title="Edit Organization"
 * />
 * ```
 * <BaseOrganizationProfile
 *   organization={organizationData}
 *   fields={[
 *     { key: 'id', label: 'Organization ID' },
 *     { key: 'name', label: 'Organization Name' },
 *     { key: 'description', label: 'Description', render: (value) => value || 'No description' },
 *     { key: 'created', label: 'Created Date', render: (value) => new Date(value).toLocaleDateString() },
 *     { key: 'attributes', label: 'Custom Attributes' }
 *   ]}
 * />
 * ```
 */
const BaseOrganizationProfile: FC<BaseOrganizationProfileProps> = ({
  fallback = null,
  className = '',
  cardLayout = true,
  organization,
  title = 'Organization Profile',
  mode = 'inline',
  editable = true,
  onOpenChange,
  onUpdate,
  open = false,
  saveButtonText = 'Save Changes',
  cancelButtonText = 'Cancel',
  fields = [
    {
      editable: false,
      key: 'id',
      label: 'Organization ID',
    },
    {
      editable: true,
      key: 'name',
      label: 'Organization Name',
    },
    {
      editable: true,
      key: 'description',
      label: 'Organization Description',
      render: (value: any): any => value || '-',
    },
    {
      editable: false,
      key: 'created',
      label: 'Created Date',
      render: (value: any): any => formatDate(value),
    },
    {
      editable: false,
      key: 'lastModified',
      label: 'Last Modified Date',
      render: (value: any): any => formatDate(value),
    },
  ],
}: BaseOrganizationProfileProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const styles: Record<string, string> = useStyles(theme, colorScheme);
  const [editedOrganization, setEditedOrganization] = useState(organization);
  const [editingFields, setEditingFields] = useState<Record<string, boolean>>({});

  const PencilIcon = (): ReactElement => (
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d="M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z" />
    </svg>
  );

  const toggleFieldEdit: (fieldName: string) => void = useCallback((fieldName: string) => {
    setEditingFields((prev: Record<string, boolean>) => ({
      ...prev,
      [fieldName]: !prev[fieldName],
    }));
  }, []);

  const getFieldPlaceholder: (fieldKey: string) => string = useCallback((fieldKey: string): string => {
    const fieldLabels: Record<string, string> = {
      description: 'organization description',
      name: 'organization name',
      orgHandle: 'organization handle',
      status: 'organization status',
      type: 'organization type',
    };

    const fieldLabel: string = fieldLabels[fieldKey] || fieldKey.toLowerCase();
    return `Enter ${fieldLabel}`;
  }, []);

  const handleFieldSave: (fieldKey: string) => void = useCallback(
    (fieldKey: string): void => {
      if (!onUpdate || !fieldKey) return;

      let fieldValue: any;
      if (editedOrganization && fieldKey && editedOrganization[fieldKey as keyof OrganizationDetails] !== undefined) {
        fieldValue = editedOrganization[fieldKey as keyof OrganizationDetails];
      } else if (organization?.[fieldKey as keyof OrganizationDetails] !== undefined) {
        fieldValue = organization[fieldKey as keyof OrganizationDetails];
      } else {
        fieldValue = '';
      }

      const payload: Record<string, any> = {
        [fieldKey]: fieldValue,
      };

      onUpdate(payload);
      toggleFieldEdit(fieldKey);
    },
    [editedOrganization, organization, onUpdate, toggleFieldEdit],
  );

  const handleFieldCancel: (fieldKey: string) => void = useCallback(
    (fieldKey: string) => {
      setEditedOrganization((prev: any) => ({
        ...prev,
        [fieldKey]: organization?.[fieldKey as keyof OrganizationDetails],
      }));
      toggleFieldEdit(fieldKey);
    },
    [organization, toggleFieldEdit],
  );

  const getOrgInitials = (name?: string): string => {
    if (!name) return 'ORG';
    return name
      .split(' ')
      .map((word: string) => word.charAt(0))
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  const renderField = (
    field: any,
    isEditing: boolean,
    onEditValue?: (value: any) => void,
    onStartEdit?: () => void,
  ): ReactElement | null => {
    if (!field) return null;

    const {key, label, editable: fieldEditable = true} = field;
    const value: any =
      key === 'attributes' ? organization?.attributes || {} : organization?.[key as keyof OrganizationDetails];

    const renderedValue: any = field.render ? field.render(value, organization) : value;

    if (isEditing && onEditValue && fieldEditable && editable) {
      const fieldValue: any =
        editedOrganization && key && editedOrganization[key as keyof OrganizationDetails] !== undefined
          ? editedOrganization[key as keyof OrganizationDetails]
          : value || '';

      const commonProps: any = {
        className: cx(styles['fieldInput']),
        label: undefined,
        onChange: (e: any): void => onEditValue(e.target ? e.target.value : e),
        placeholder: getFieldPlaceholder(key),
        value: typeof fieldValue === 'object' ? JSON.stringify(fieldValue) : String(fieldValue || ''),
      };

      let fieldInput: ReactElement;

      if (key === 'attributes') {
        const attributesValue: any = typeof fieldValue === 'object' && fieldValue !== null ? fieldValue : {};
        fieldInput = (
          <KeyValueInput
            value={attributesValue}
            onChange={(pairs: any): void => {
              const attributesObject: Record<string, any> = pairs.reduce(
                (acc: any, pair: any) => {
                  acc[pair.key] = pair.value;
                  return acc;
                },
                {} as Record<string, any>,
              );
              onEditValue(attributesObject);
            }}
            onAdd={(pair: any): void => {
              if (onUpdate) {
                const operation: any = {
                  operation: 'ADD',
                  path: `/attributes/${pair.key}`,
                  value: pair.value,
                };
                onUpdate([operation]);
              }
            }}
            onRemove={(pair: any): void => {
              if (onUpdate) {
                const operation: any = {
                  operation: 'REMOVE',
                  path: `/attributes/${pair.key}`,
                  value: '',
                };
                onUpdate([operation]);
              }
            }}
            label=""
            keyPlaceholder="Attribute name"
            valuePlaceholder="Attribute value"
            helperText="Add custom attributes as key-value pairs"
          />
        );
      } else {
        fieldInput = <TextField {...commonProps} />;
      }

      return (
        <>
          <span className={cx(styles['label'])}>{label}</span>
          <div className={cx(styles['value'])}>{fieldInput}</div>
        </>
      );
    }

    const hasValue: boolean = value !== undefined && value !== null && value !== '';
    const isFieldEditable: boolean = editable && fieldEditable;

    let displayValue: string | ReactElement;
    if (hasValue) {
      displayValue =
        key === 'attributes' && typeof value === 'object' && value !== null ? (
          <KeyValueInput value={value} readOnly={true} label="" />
        ) : (
          String(renderedValue)
        );
    } else if (isFieldEditable) {
      displayValue = getFieldPlaceholder(key);
    } else {
      displayValue = '-';
    }

    return (
      <>
        <span className={cx(styles['label'])}>{label}</span>
        <div className={cx(styles['value'], !hasValue && styles['valueEmpty'])}>
          {!hasValue && isFieldEditable && onStartEdit ? (
            <Button
              onClick={onStartEdit}
              variant="text"
              color="secondary"
              size="small"
              title="Click to edit"
              className={cx(styles['placeholderButton'])}
            >
              {displayValue}
            </Button>
          ) : (
            displayValue
          )}
        </div>
      </>
    );
  };

  const renderOrganizationField = (field: any): ReactElement | null => {
    if (!field?.key) return null;

    const hasValue: boolean =
      organization?.[field.key as keyof OrganizationDetails] !== undefined &&
      organization?.[field.key as keyof OrganizationDetails] !== '' &&
      organization?.[field.key as keyof OrganizationDetails] !== null;
    const isFieldEditing: boolean = editingFields[field.key];
    const isFieldEditable: boolean = editable && field.editable !== false;

    const shouldShow: boolean = hasValue || isFieldEditing || isFieldEditable;

    if (!shouldShow) {
      return null;
    }

    return (
      <div className={cx(styles['field'])} key={field.key}>
        <div className={cx(styles['fieldContent'])}>
          {renderField(
            field,
            isFieldEditing,
            (value: any): void => {
              const tempEditedOrganization: any = {...editedOrganization};
              tempEditedOrganization[field.key as keyof OrganizationDetails] = value;
              setEditedOrganization(tempEditedOrganization);
            },
            (): void => toggleFieldEdit(field.key),
          )}
        </div>
        {isFieldEditable && (
          <div className={cx(styles['fieldActions'])}>
            {isFieldEditing ? (
              <>
                <Button
                  onClick={(): void => handleFieldSave(field.key)}
                  color="primary"
                  variant="solid"
                  size="small"
                  title="Save changes"
                >
                  {saveButtonText}
                </Button>
                <Button
                  onClick={(): void => handleFieldCancel(field.key)}
                  color="secondary"
                  variant="solid"
                  size="small"
                  title="Cancel editing"
                >
                  {cancelButtonText}
                </Button>
              </>
            ) : (
              hasValue && (
                <Button
                  onClick={(): void => toggleFieldEdit(field.key)}
                  variant="text"
                  color="secondary"
                  size="small"
                  title="Edit field"
                  className={cx(styles['editButton'])}
                >
                  <PencilIcon />
                </Button>
              )
            )}
          </div>
        )}
      </div>
    );
  };

  if (!organization) {
    return fallback;
  }

  const profileContent: ReactElement = (
    <CardPrimitive className={cx(styles['root'], cardLayout && styles['card'], className)}>
      <div className={cx(styles['header'])}>
        <Avatar name={getOrgInitials(organization.name)} size={80} alt={`${organization.name} logo`} />
        <div className={cx(styles['orgInfo'])}>
          <h2 className={cx(styles['name'])}>{organization.name}</h2>
          {organization.orgHandle && <p className={cx(styles['handle'])}>@{organization.orgHandle}</p>}
        </div>
      </div>

      <div className={cx(styles['infoContainer'])}>{fields.map((field: any) => renderOrganizationField(field))}</div>
    </CardPrimitive>
  );

  if (mode === 'popup') {
    return (
      <DialogPrimitive open={open} onOpenChange={onOpenChange}>
        <DialogPrimitive.Content>
          <DialogPrimitive.Heading>{title}</DialogPrimitive.Heading>
          <div className={cx(styles['popup'])}>{profileContent}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive>
    );
  }

  return profileContent;
};

export default BaseOrganizationProfile;
