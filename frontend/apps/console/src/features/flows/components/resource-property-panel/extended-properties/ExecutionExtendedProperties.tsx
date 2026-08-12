// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack} from '@wso2/oxygen-ui';
import {useCallback, useMemo, type ReactNode} from 'react';
import ConsentProperties from './execution-properties/ConsentProperties';
import {EXECUTOR_TO_IDP_TYPE_MAP, EXECUTORS_WITH_FIXED_INPUTS} from './execution-properties/constants';
import EmailProperties from './execution-properties/EmailProperties';
import ExecutorInputsEditor from './execution-properties/ExecutorInputsEditor';
import FederationProperties from './execution-properties/FederationProperties';
import HttpRequestProperties from './execution-properties/HttpRequestProperties';
import IdentifyingProperties from './execution-properties/IdentifyingProperties';
import InviteProperties from './execution-properties/InviteProperties';
import MagicLinkProperties from './execution-properties/MagicLinkProperties';
import NoConfigProperties from './execution-properties/NoConfigProperties';
import OpenID4VPProperties from './execution-properties/OpenID4VPProperties';
import OtpProperties from './execution-properties/OtpProperties';
import OUExecutorProperties from './execution-properties/OUExecutorProperties';
import OUResolverProperties from './execution-properties/OUResolverProperties';
import PasskeyProperties from './execution-properties/PasskeyProperties';
import PermissionValidatorProperties from './execution-properties/PermissionValidatorProperties';
import PreDeleteProperties from './execution-properties/PreDeleteProperties';
import ProvisioningProperties from './execution-properties/ProvisioningProperties';
import SessionSignOutProperties from './execution-properties/SessionSignOutProperties';
import SmsProperties from './execution-properties/SmsProperties';
import SsoCheckProperties from './execution-properties/SsoCheckProperties';
import UserTypeResolverProperties from './execution-properties/UserTypeResolverProperties';
import type {CommonResourcePropertiesPropsInterface} from '@/features/flows/components/resource-property-panel/CommonResourceProperties';
import type {FlowNodeInput} from '@/features/flows/models/responses';
import {ExecutionTypes} from '@/features/flows/models/steps';
import type {StepData} from '@/features/flows/models/steps';

/**
 * Props interface of {@link ExecutionExtendedProperties}
 */
export type ExecutionExtendedPropertiesPropsInterface = CommonResourcePropertiesPropsInterface;

/**
 * Extended properties for execution step elements.
 * Routes to the appropriate sub-component based on executor type.
 *
 * @param props - Props injected to the component.
 * @returns The ExecutionExtendedProperties component.
 */
function ExecutionExtendedProperties({resource, onChange}: ExecutionExtendedPropertiesPropsInterface): ReactNode {
  const executorName = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return stepData?.action?.executor?.name;
  }, [resource]);

  const currentInputs = useMemo((): FlowNodeInput[] => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {inputs?: FlowNodeInput[]} | undefined)?.inputs ?? [];
  }, [resource]);

  const handleInputsChange = useCallback(
    (inputs: FlowNodeInput[]) => {
      onChange('data.action.executor.inputs', inputs.length > 0 ? inputs : undefined, resource);
    },
    [onChange, resource],
  );

  if (!executorName) {
    return null;
  }

  const showInputsEditor = !EXECUTORS_WITH_FIXED_INPUTS.has(executorName);

  let executorSpecificProperties: ReactNode = null;

  switch (executorName) {
    case ExecutionTypes.ConsentExecutor:
      executorSpecificProperties = <ConsentProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.IdentifyingExecutor:
      executorSpecificProperties = <IdentifyingProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.PasskeyAuth:
      executorSpecificProperties = <PasskeyProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.OUResolverExecutor:
      executorSpecificProperties = <OUResolverProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.InviteExecutor:
      executorSpecificProperties = <InviteProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.EmailExecutor:
      executorSpecificProperties = <EmailProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.SMSExecutor:
      executorSpecificProperties = <SmsProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.OTPExecutor:
      executorSpecificProperties = <OtpProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.PermissionValidator:
      executorSpecificProperties = <PermissionValidatorProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.PreDelete:
      executorSpecificProperties = <PreDeleteProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.ProvisioningExecutor:
      executorSpecificProperties = <ProvisioningProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.OUExecutor:
      executorSpecificProperties = <OUExecutorProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.UserTypeResolver:
      executorSpecificProperties = <UserTypeResolverProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.HTTPRequestExecutor:
      executorSpecificProperties = <HttpRequestProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.MagicLinkExecutor:
      executorSpecificProperties = <MagicLinkProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.OpenID4VPVerify:
      executorSpecificProperties = <OpenID4VPProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.SSOCheck:
      executorSpecificProperties = <SsoCheckProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.SessionSignOut:
      executorSpecificProperties = <SessionSignOutProperties resource={resource} onChange={onChange} />;
      break;
    case ExecutionTypes.CredentialSetter:
    case ExecutionTypes.AttributeUniquenessValidator:
    case ExecutionTypes.Session:
      executorSpecificProperties = <NoConfigProperties />;
      break;
    default:
      // Federated executors (Google, GitHub, OAuth, OIDC) - check if executor has an IDP type mapping
      if (EXECUTOR_TO_IDP_TYPE_MAP[executorName]) {
        executorSpecificProperties = <FederationProperties resource={resource} onChange={onChange} />;
      }
      break;
  }

  return (
    <Stack gap={2}>
      {executorSpecificProperties}
      {showInputsEditor && <ExecutorInputsEditor inputs={currentInputs} onChange={handleInputsChange} />}
    </Stack>
  );
}

export default ExecutionExtendedProperties;
