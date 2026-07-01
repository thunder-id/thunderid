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

import {render, screen} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import MaskedSecretField from '../MaskedSecretField';

vi.mock('react-i18next', () => ({useTranslation: () => ({t: (key: string) => key})}));

describe('MaskedSecretField', () => {
  it('shows the editable hint when replacing or creating a secret', () => {
    render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={vi.fn()}
        hasStoredSecret={false}
        replacing={false}
        onReplacingChange={vi.fn()}
        hint="connections:form.fields.clientSecret.hint"
      />,
    );

    expect(screen.getByText('connections:form.fields.clientSecret.hint')).toBeInTheDocument();
  });

  it('keeps the stored-secret helper when the secret is not being replaced', () => {
    render(
      <MaskedSecretField
        id="client-secret"
        label="Client secret"
        value=""
        onChange={vi.fn()}
        hasStoredSecret
        replacing={false}
        onReplacingChange={vi.fn()}
        hint="connections:form.fields.clientSecret.hint"
      />,
    );

    expect(screen.getByText('form.secret.keepHelp')).toBeInTheDocument();
    expect(screen.queryByText('form.fields.clientSecret.hint')).not.toBeInTheDocument();
  });
});
