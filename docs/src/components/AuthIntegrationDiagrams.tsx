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

import { SequenceDiagram } from './SequenceDiagram';

export function RedirectBasedDiagram() {
  // Actors: 0 = User, 1 = User Agent, 2 = Application, 3 = ThunderID
  return (
    <SequenceDiagram
      actors={['User', 'User Agent', 'Application', 'ThunderID']}
      gaps={[145, 235, 245]}
      ariaLabel="Redirect-based OAuth 2.1 flow: User initiates login, application redirects user agent to ThunderID for authentication, user submits credentials, ThunderID redirects back with an authorization code, and the application exchanges it for tokens."
      rows={[
        { from: 0, to: 2, label: 'Initiate login' },
        { from: 2, to: 1, label: ['Redirect to', '/oauth2/authorize'] },
        { from: 1, to: 3, label: 'GET /oauth2/authorize', sublabel: 'response_type=code&client_id=...' },
        { from: 3, to: 1, label: 'Render sign-in page' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 3, label: 'POST credentials' },
        { from: 3, to: 1, label: ['Redirect: ?code=...&state=...'] },
        { from: 1, to: 2, label: ['Callback with', 'authorization code'] },
        { from: 2, to: 3, label: 'POST /oauth2/token', sublabel: '(authorization code)' },
        { from: 3, to: 2, label: ['Access token, ID token,', 'refresh token'] },
      ]}
    />
  );
}

export function AppNativeDiagram() {
  // Actors: 0 = User, 1 = Application, 2 = ThunderID
  return (
    <SequenceDiagram
      actors={['User', 'Application', 'ThunderID']}
      gaps={[380, 245]}
      ariaLabel="App-native flow: User interacts with the application, which calls the Flow Execution API to advance authentication steps, rendering each step locally."
      rows={[
        { from: 0, to: 1, label: 'Initiate login' },
        { from: 1, to: 2, label: 'POST /flow/execute (start)' },
        { from: 2, to: 1, label: ['Step 1: Collect', 'username/password'] },
        { from: 1, to: 0, label: 'Render login form' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 2, label: ['POST /flow/execute', '(credentials)'] },
        { from: 2, to: 1, label: 'Step 2: Collect OTP' },
        { from: 1, to: 0, label: 'Render OTP form' },
        { from: 0, to: 1, label: 'Submit OTP' },
        { from: 1, to: 2, label: 'POST /flow/execute (OTP)' },
        { from: 2, to: 1, label: ['Flow complete:', 'assertion token'] },
      ]}
    />
  );
}

export function DirectAPIDiagram() {
  // Actors: 0 = User, 1 = Application, 2 = ThunderID
  return (
    <SequenceDiagram
      actors={['User', 'Application', 'ThunderID']}
      gaps={[380, 245]}
      ariaLabel="Direct API flow: User submits credentials to the application, which calls individual authentication endpoints on ThunderID, chaining assertion tokens for step-up authentication."
      rows={[
        { from: 0, to: 1, label: 'Initiate login' },
        { from: 1, to: 0, label: 'Render credentials form' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 2, label: ['POST /auth/credentials', '/authenticate'] },
        { from: 2, to: 1, label: ['User details +', 'assertion token'] },
        { from: 1, to: 2, label: 'POST /auth/otp/sms/send' },
        { from: 2, to: 1, label: 'Session token' },
        { from: 1, to: 0, label: 'Prompt for OTP' },
        { from: 0, to: 1, label: 'Submit OTP' },
        { from: 1, to: 2, label: 'POST /auth/otp/sms/verify', sublabel: '(with previous assertion token)' },
        { from: 2, to: 1, label: 'Enriched assertion token' },
      ]}
    />
  );
}
