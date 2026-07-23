/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import type {JSX} from 'react';
import TrustedIssuerCreateForm from '../components/TrustedIssuerCreateForm';

interface TrustedIssuerWizardStepProps {
  /** Connection name collected on the wizard's name step. */
  name: string;
  /** Call when the create request 409s on a duplicate name, to bounce back to the name step. */
  onNameConflict: () => void;
}

/**
 * The "trusted-idp" configure step plugged into the "Add custom connection" wizard (see
 * `customConfigureSteps` on `ConnectionCreateWizardPage`). Renders the trusted-issuer create
 * form; the wizard supplies the surrounding chrome (breadcrumb, progress, Back, close) and the
 * name, collected on its own name step.
 */
export default function TrustedIssuerWizardStep({name, onNameConflict}: TrustedIssuerWizardStepProps): JSX.Element {
  return <TrustedIssuerCreateForm name={name} onNameConflict={onNameConflict} />;
}
