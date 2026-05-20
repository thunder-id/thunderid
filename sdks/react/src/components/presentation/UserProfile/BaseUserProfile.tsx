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
import {User, withVendorCSSClassPrefix, WellKnownSchemaIds, bem, Preferences} from '@thunderid/browser';
import {FC, ReactElement, useState, useCallback} from 'react';
import useStyles from './BaseUserProfile.styles';
import useTheme from '../../../contexts/Theme/useTheme';
import useTranslation from '../../../hooks/useTranslation';
import getDisplayName from '../../../utils/getDisplayName';
import getMappedUserProfileValue from '../../../utils/getMappedUserProfileValue';
import AlertPrimitive from '../../primitives/Alert/Alert';
import {Avatar} from '../../primitives/Avatar/Avatar';
import Button from '../../primitives/Button/Button';
import CardPrimitive from '../../primitives/Card/Card';
import Checkbox from '../../primitives/Checkbox/Checkbox';
import DatePicker from '../../primitives/DatePicker/DatePicker';
import DialogPrimitive from '../../primitives/Dialog/Dialog';
import Divider from '../../primitives/Divider/Divider';
import MultiInput from '../../primitives/MultiInput/MultiInput';
import TextField from '../../primitives/TextField/TextField';
import Typography from '../../primitives/Typography/Typography';

interface ExtendedFlatSchema {
  path?: string;
  schemaId?: string;
}

interface Schema extends ExtendedFlatSchema {
  caseExact?: boolean;
  description?: string;
  displayName?: string;
  displayOrder?: string;
  multiValued?: boolean;
  mutability?: string;
  name?: string;
  required?: boolean;
  returned?: string;
  subAttributes?: Schema[];
  type?: string;
  uniqueness?: string;
  value?: any;
}

export interface BaseUserProfileProps {
  attributeMapping?: {
    [key: string]: string | string[] | undefined;
    firstName?: string | string[];
    lastName?: string | string[];
    picture?: string | string[];
    username?: string | string[];
  };
  cardLayout?: boolean;
  className?: string;
  displayNameAttributes?: string[];
  editable?: boolean;
  error?: string | null;
  fallback?: ReactElement;
  flattenedProfile?: User;
  hideFields?: string[];
  isLoading?: boolean;
  mode?: 'inline' | 'popup';
  onOpenChange?: (open: boolean) => void;
  onUpdate?: (payload: any) => Promise<void>;
  open?: boolean;
  /**
   * Component-level preferences to override global i18n and theme settings.
   * Preferences are deep-merged with global ones, with component preferences
   * taking precedence. Affects this component and all its descendants.
   */
  preferences?: Preferences;
  profile?: User;
  schemas?: Schema[];
  showFields?: string[];

  title?: string;
}

// Fields to skip based on schema.name
const fieldsToSkip: string[] = [
  'roles.default',
  'active',
  'groups',
  'accountLocked',
  'accountDisabled',
  'oneTimePassword',
  'userSourceId',
  'idpType',
  'localCredentialExists',
  'active',
  'ResourceType',
  'ExternalID',
  'MetaData',
  'verifiedMobileNumbers',
  'verifiedEmailAddresses',
  'phoneNumbers.mobile',
  'emailAddresses',
  'preferredMFAOption',
];

// Fields that should be readonly
const readonlyFields: string[] = ['username', 'userName', 'user_name'];

const BaseUserProfile: FC<BaseUserProfileProps> = ({
  fallback = null,
  className = '',
  cardLayout = true,
  profile,
  schemas = [],
  flattenedProfile,
  mode = 'inline',
  title,
  attributeMapping = {},
  editable = true,
  onOpenChange,
  onUpdate,
  open = false,
  error = null,
  isLoading = false,
  preferences,
  showFields = [],
  hideFields = [],
  displayNameAttributes = [],
}: BaseUserProfileProps): ReactElement => {
  const {theme, colorScheme} = useTheme();
  const [editedUser, setEditedUser] = useState(flattenedProfile || profile);
  const [editingFields, setEditingFields] = useState<Record<string, boolean>>({});
  const {t} = useTranslation(preferences?.i18n);

  /**
   * Determines if a field should be visible based on showFields, hideFields, and fieldsToSkip arrays.
   * Priority order:
   * 1. fieldsToSkip (always hidden) - highest priority
   * 2. hideFields (explicitly hidden)
   * 3. showFields (explicitly shown, if array is not empty)
   * 4. Default behavior (show all fields not in fieldsToSkip)
   */
  const shouldShowField: any = useCallback(
    (fieldName: string): boolean => {
      // Always skip fields in the hardcoded fieldsToSkip array
      if (fieldsToSkip.includes(fieldName)) {
        return false;
      }

      // If hideFields is provided and contains this field, hide it
      if (hideFields.length > 0 && hideFields.includes(fieldName)) {
        return false;
      }

      // If showFields is provided and not empty, only show fields in that array
      if (showFields.length > 0) {
        return showFields.includes(fieldName);
      }

      return true;
    },
    [showFields, hideFields],
  );

  const PencilIcon = (): any => (
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

  const toggleFieldEdit: any = useCallback((fieldName: string) => {
    setEditingFields((prev: any) => ({
      ...prev,
      [fieldName]: !prev[fieldName],
    }));
  }, []);

  const getFieldPlaceholder: any = useCallback((schema: Schema): string => {
    const {type, displayName, description, name} = schema;

    const fieldLabel: any = displayName || description || name || 'value';

    switch (type) {
      case 'DATE_TIME':
        return `Enter your ${fieldLabel.toLowerCase()}`;
      case 'BOOLEAN':
        return `Select ${fieldLabel.toLowerCase()}`;
      case 'COMPLEX':
        return `Enter ${fieldLabel.toLowerCase()} details`;
      default:
        return `Enter your ${fieldLabel.toLowerCase()}`;
    }
  }, []);

  const formatLabel: any = useCallback(
    (key: string): string =>
      key
        .split(/(?=[A-Z])|_/)
        .map((word: any) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
        .join(' '),
    [],
  );

  const styles: any = useStyles(theme, colorScheme);

  const ObjectDisplay: FC<{data: unknown}> = ({data}: {data: unknown}): ReactElement => {
    if (!data || typeof data !== 'object') return null;

    return (
      <table className={styles.value}>
        <tbody>
          {Object.entries(data).map(([key, value]: any) => (
            <tr key={key}>
              <td className={styles.objectKey}>
                <strong>{formatLabel(key)}:</strong>
              </td>
              <td className={styles.objectValue}>
                {typeof value === 'object' ? <ObjectDisplay data={value} /> : String(value)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    );
  };

  function set(obj: Record<string, any>, path: string, value: any): void {
    const keys: any = path.split('.');
    let current: any = obj;

    for (let i: any = 0; i < keys.length; i += 1) {
      const key: any = keys[i];

      if (i === keys.length - 1) {
        current[key] = value;
      } else {
        if (!current[key] || typeof current[key] !== 'object') {
          current[key] = {};
        }
        current = current[key];
      }
    }
  }

  const handleFieldSave: any = useCallback(
    (schema: Schema): void => {
      if (!onUpdate || !schema.name) return;

      const fieldName: string = schema.name;
      let fieldValue: any;
      if (editedUser && fieldName && editedUser[fieldName] !== undefined) {
        fieldValue = editedUser[fieldName];
      } else if (flattenedProfile?.[fieldName] !== undefined) {
        fieldValue = flattenedProfile[fieldName];
      } else {
        fieldValue = '';
      }

      if (Array.isArray(fieldValue)) {
        fieldValue = fieldValue.filter((v: any) => v !== undefined && v !== null && v !== '');
      }

      let payload: Record<string, any> = {};

      // SCIM Patch Operation Logic:
      // - Fields from core schema (urn:ietf:params:scim:schemas:core:2.0:User)
      //   should be sent directly: {"name":{"givenName":"John"}}
      // - Fields from extension schemas (like urn:scim:wso2:schema)
      //   should be nested under the schema namespace: {"urn:scim:wso2:schema":{"country":"Sri Lanka"}}
      if (schema.schemaId && schema.schemaId !== WellKnownSchemaIds.User) {
        // For non-core schemas, nest the field under the schema namespace
        payload = {
          [schema.schemaId]: {
            [fieldName]: fieldValue,
          },
        };
      } else {
        // For core schema or fields without schemaId, use the field path directly
        // This handles complex paths like "name.givenName" correctly
        set(payload, fieldName, fieldValue);
      }

      onUpdate(payload);

      toggleFieldEdit(fieldName);
    },
    [editedUser, flattenedProfile, onUpdate, toggleFieldEdit],
  );

  const handleFieldCancel: any = useCallback(
    (fieldName: string) => {
      const currentUser: any = flattenedProfile || profile;
      setEditedUser((prev: any) => ({
        ...prev,
        [fieldName]: currentUser[fieldName],
      }));
      toggleFieldEdit(fieldName);
    },
    [flattenedProfile, profile, toggleFieldEdit],
  );

  const defaultAttributeMappings: Record<string, string | string[] | undefined> = {
    email: ['emails', 'email'],
    firstName: ['name.givenName', 'given_name'],
    lastName: ['name.familyName', 'family_name'],
    picture: ['profile', 'profileUrl', 'picture', 'URL'],
    username: ['userName', 'username', 'user_name'],
  };

  const mergedMappings: Record<string, string | string[] | undefined> = {
    ...defaultAttributeMappings,
    ...attributeMapping,
  };

  const renderSchemaField = (
    schema: Schema,
    isEditing: boolean,
    onEditValue?: (value: any) => void,
    onStartEdit?: () => void,
  ): ReactElement | null => {
    if (!schema) return null;
    const {value, displayName, description, name, type, required, mutability, subAttributes, multiValued} = schema;
    const label: any = displayName || description || name || '';

    if (subAttributes && Array.isArray(subAttributes)) {
      return (
        <>
          {subAttributes.map((subAttr: any, index: any) => {
            let displayValue: string;
            if (Array.isArray(subAttr.value)) {
              displayValue = subAttr.value
                .map((item: any) => (typeof item === 'object' ? JSON.stringify(item) : String(item)))
                .join(', ');
            } else if (typeof subAttr.value === 'object') {
              displayValue = JSON.stringify(subAttr.value);
            } else {
              displayValue = String(subAttr.value);
            }

            return (
              <div key={index} className={styles.field}>
                <span className={styles.label}>{subAttr.displayName || subAttr.description || ''}</span>
                <div className={styles.value}>{displayValue}</div>
              </div>
            );
          })}
        </>
      );
    }

    if (Array.isArray(value) || multiValued) {
      const hasValues: any = Array.isArray(value)
        ? value.length > 0
        : value !== undefined && value !== null && value !== '';
      const isEditable: any = editable && mutability !== 'READ_ONLY' && !readonlyFields.includes(name || '');

      if (isEditing && onEditValue && isEditable) {
        let currentValue: any;
        if (editedUser && name && editedUser[name] !== undefined) {
          currentValue = editedUser[name];
        } else if (flattenedProfile && name && flattenedProfile[name] !== undefined) {
          currentValue = flattenedProfile[name];
        } else {
          currentValue = value;
        }

        let fieldValues: string[];
        if (Array.isArray(currentValue)) {
          fieldValues = currentValue.map(String);
        } else if (currentValue !== undefined && currentValue !== null && currentValue !== '') {
          fieldValues = [String(currentValue)];
        } else {
          fieldValues = [];
        }

        return (
          <>
            <span className={styles.label}>{label}</span>
            <div className={styles.value}>
              <MultiInput
                values={fieldValues}
                onChange={(newValues: any): void => {
                  if (multiValued || Array.isArray(currentValue)) {
                    onEditValue(newValues);
                  } else {
                    onEditValue(newValues[0] || '');
                  }
                }}
                placeholder={getFieldPlaceholder(schema)}
                fieldType={type as 'STRING' | 'DATE_TIME' | 'BOOLEAN'}
                type={type === 'DATE_TIME' ? 'date' : 'text'}
                required={required}
              />
            </div>
          </>
        );
      }

      let displayValue: string;
      if (hasValues) {
        if (Array.isArray(value)) {
          displayValue = value
            .map((item: any) => (typeof item === 'object' ? JSON.stringify(item) : String(item)))
            .join(', ');
        } else {
          displayValue = String(value);
        }
      } else if (isEditable) {
        displayValue = getFieldPlaceholder(schema);
      } else {
        displayValue = '-';
      }

      return (
        <>
          <span className={styles.label}>{label}</span>
          <div className={cx(styles.value, !hasValues ? styles.valuePlaceholder : '')}>
            {!hasValues && isEditable && onStartEdit ? (
              <Button
                onClick={onStartEdit}
                variant="text"
                color="secondary"
                size="small"
                title="Click to edit"
                className={styles.editButton}
              >
                {displayValue}
              </Button>
            ) : (
              displayValue
            )}
          </div>
        </>
      );
    }
    if (type === 'COMPLEX' && typeof value === 'object') {
      return <ObjectDisplay data={value} />;
    }

    if (isEditing && onEditValue && mutability !== 'READ_ONLY' && !readonlyFields.includes(name || '')) {
      let fieldValue: any;
      if (editedUser && name && editedUser[name] !== undefined) {
        fieldValue = editedUser[name];
      } else if (flattenedProfile && name && flattenedProfile[name] !== undefined) {
        fieldValue = flattenedProfile[name];
      } else {
        fieldValue = value || '';
      }

      const commonProps: any = {
        label: undefined,
        onChange: (e: any) => onEditValue(e.target ? e.target.value : e),
        placeholder: getFieldPlaceholder(schema),
        required,
        value: fieldValue,
        // Removed inline style, use .styles.ts for marginBottom if needed
      };
      let field: ReactElement;
      switch (type) {
        case 'STRING':
          field = <TextField {...commonProps} />;
          break;
        case 'DATE_TIME':
          field = <DatePicker {...commonProps} />;
          break;
        case 'BOOLEAN':
          field = (
            <Checkbox
              {...commonProps}
              checked={!!fieldValue}
              onChange={(e: any): void => {
                onEditValue(e.target.checked);
              }}
            />
          );
          break;
        case 'COMPLEX':
          field = (
            <textarea
              value={fieldValue}
              onChange={(e: any): void => onEditValue(e.target.value)}
              placeholder={getFieldPlaceholder(schema)}
              required={required}
              className={styles.complexTextarea}
            />
          );
          break;
        default:
          field = <TextField {...commonProps} />;
      }
      return (
        <>
          <span className={styles.label}>{label}</span>
          <div className={styles.value}>{field}</div>
        </>
      );
    }

    const hasValue: any = value !== undefined && value !== null && value !== '';
    const isEditable: any = editable && mutability !== 'READ_ONLY' && !readonlyFields.includes(name || '');

    let displayValue: string;
    if (hasValue) {
      displayValue = String(value);
    } else if (isEditable) {
      displayValue = getFieldPlaceholder(schema);
    } else {
      displayValue = '-';
    }

    return (
      <>
        <span className={styles.label}>{label}</span>
        <div className={cx(styles.value, !hasValue ? styles.valuePlaceholder : '')}>
          {!hasValue && isEditable && onStartEdit ? (
            <Button
              onClick={onStartEdit}
              variant="text"
              color="secondary"
              size="small"
              title="Click to edit"
              className={styles.editButton}
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

  const renderUserInfo = (schema: Schema): any => {
    if (!schema?.name) return null;

    const hasValue: any = schema.value !== undefined && schema.value !== '' && schema.value !== null;
    const isFieldEditing: any = editingFields[schema.name];
    const isReadonlyField: any = readonlyFields.includes(schema.name);

    const shouldShow: any = hasValue || isFieldEditing || (editable && schema.mutability === 'READ_WRITE');

    if (!shouldShow) {
      return null;
    }

    return (
      <div className={styles.field}>
        <div className={styles.fieldInner}>
          {renderSchemaField(
            schema,
            isFieldEditing,
            (value: any) => {
              const tempEditedUser: any = {...editedUser};
              tempEditedUser[schema.name] = value;
              setEditedUser(tempEditedUser);
            },
            () => toggleFieldEdit(schema.name),
          )}
        </div>
        {editable && schema.mutability !== 'READ_ONLY' && !isReadonlyField && (
          <div className={styles.fieldActions}>
            {isFieldEditing && (
              <>
                <Button size="small" color="primary" variant="solid" onClick={(): any => handleFieldSave(schema)}>
                  Save
                </Button>
                <Button
                  size="small"
                  color="secondary"
                  variant="solid"
                  onClick={(): any => handleFieldCancel(schema.name)}
                >
                  Cancel
                </Button>
              </>
            )}
            {!isFieldEditing && hasValue && (
              <Button
                size="small"
                color="tertiary"
                variant="icon"
                onClick={(): any => toggleFieldEdit(schema.name)}
                title="Edit"
                className={styles.editButton}
              >
                <PencilIcon />
              </Button>
            )}
          </div>
        )}
      </div>
    );
  };

  if (!profile && !flattenedProfile) {
    return fallback;
  }

  const containerClasses: any = cx(
    styles.root,
    cardLayout ? styles.card : '',
    withVendorCSSClassPrefix('user-profile'),
    className,
  );

  const currentUser: any = flattenedProfile || profile;

  const renderProfileWithoutSchemas = (): any => {
    if (!currentUser) return null;

    const displayName: any = getDisplayName(mergedMappings, profile, displayNameAttributes);

    const profileEntries: any = Object.entries(currentUser)
      .filter(([key, value]: [string, any]) => {
        if (!shouldShowField(key)) return false;

        return value !== undefined && value !== '' && value !== null;
      })
      .sort(([a]: [string, ...any[]], [b]: [string, ...any[]]) => a.localeCompare(b));

    return (
      <>
        <div className={styles.profileSummary}>
          <Avatar
            imageUrl={getMappedUserProfileValue('picture', mergedMappings, currentUser)}
            name={displayName}
            size={70}
            alt={`${displayName}'s avatar`}
            isLoading={isLoading}
          />
          <Typography variant="h3" fontWeight="medium">
            {displayName}
          </Typography>
          {getMappedUserProfileValue('email', mergedMappings, currentUser) && (
            <Typography variant="body2" color="textSecondary">
              {getMappedUserProfileValue('email', mergedMappings, currentUser)}
            </Typography>
          )}
        </div>
        <Divider />
        {profileEntries.map(([key, value]: any, index: any) => (
          <div key={key}>
            <div className={styles.sectionRow}>
              <div className={styles.sectionLabel}>{formatLabel(key)}</div>
              <div className={styles.sectionValue}>
                {typeof value === 'object' ? <ObjectDisplay data={value} /> : String(value)}
              </div>
            </div>
            {index < profileEntries.length - 1 && <Divider />}
          </div>
        ))}
      </>
    );
  };

  const profileContent: any = (
    <CardPrimitive className={containerClasses}>
      {error && (
        <AlertPrimitive
          variant="error"
          className={cx(withVendorCSSClassPrefix(bem('user-profile', 'alert')), styles.alert)}
        >
          <AlertPrimitive.Title>{t('errors.heading') || 'Error'}</AlertPrimitive.Title>
          <AlertPrimitive.Description>{error}</AlertPrimitive.Description>
        </AlertPrimitive>
      )}
      {schemas && schemas.length > 0 && (
        <div className={styles.header}>
          <Avatar
            imageUrl={getMappedUserProfileValue('picture', mergedMappings, currentUser)}
            name={getDisplayName(mergedMappings, profile)}
            size={80}
            alt={`${getDisplayName(mergedMappings, profile)}'s avatar`}
            isLoading={isLoading}
          />
        </div>
      )}
      <div className={styles.infoContainer}>
        {schemas && schemas.length > 0
          ? schemas
              .filter((schema: any) => {
                if (!schema.name || !shouldShowField(schema.name)) return false;

                if (!editable) {
                  const value: any = flattenedProfile && schema.name ? flattenedProfile[schema.name] : undefined;
                  return value !== undefined && value !== '' && value !== null;
                }

                return true;
              })
              .sort((a: any, b: any) => {
                const orderA: any = a.displayOrder ? parseInt(a.displayOrder, 10) : 999;
                const orderB: any = b.displayOrder ? parseInt(b.displayOrder, 10) : 999;
                return orderA - orderB;
              })
              .map((schema: any, index: any) => {
                const value: any = flattenedProfile && schema.name ? flattenedProfile[schema.name] : undefined;
                const schemaWithValue: any = {
                  ...schema,
                  value,
                };

                return (
                  <div key={schema.name || index} className={styles.info}>
                    {renderUserInfo(schemaWithValue)}
                  </div>
                );
              })
          : renderProfileWithoutSchemas()}
      </div>
    </CardPrimitive>
  );

  if (mode === 'popup') {
    return (
      <DialogPrimitive open={open} onOpenChange={onOpenChange}>
        <DialogPrimitive.Content>
          <DialogPrimitive.Heading>{title ?? t('user.profile.heading')}</DialogPrimitive.Heading>
          <div className={styles.popup}>{profileContent}</div>
        </DialogPrimitive.Content>
      </DialogPrimitive>
    );
  }

  return profileContent;
};

export default BaseUserProfile;
