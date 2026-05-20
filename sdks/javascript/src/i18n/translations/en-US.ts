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

import {I18nTranslations, I18nMetadata, I18nBundle} from '../models/i18n';

const translations: I18nTranslations = {
  /* |---------------------------------------------------------------| */
  /* |                        Elements                               | */
  /* |---------------------------------------------------------------| */

  /* Buttons */
  'elements.buttons.signin.text': 'Sign In',
  'elements.buttons.signout.text': 'Sign Out',
  'elements.buttons.signup.text': 'Sign Up',
  'elements.buttons.submit.text': 'Continue',
  'elements.buttons.facebook.text': 'Continue with Facebook',
  'elements.buttons.google.text': 'Continue with Google',
  'elements.buttons.github.text': 'Continue with GitHub',
  'elements.buttons.microsoft.text': 'Continue with Microsoft',
  'elements.buttons.linkedin.text': 'Continue with LinkedIn',
  'elements.buttons.ethereum.text': 'Continue with Sign In Ethereum',
  'elements.buttons.smsotp.text': 'Continue with SMS OTP',
  'elements.buttons.multi.option.text': 'Continue with {connection}',
  'elements.buttons.social.text': 'Continue with {connection}',

  /* Display */
  'elements.display.divider.or_separator': 'OR',
  'elements.display.copyable_text.copy': 'Copy',
  'elements.display.copyable_text.copied': 'Copied!',

  /* Fields */
  'elements.fields.generic.placeholder': 'Enter your {field}',
  'elements.fields.username.label': 'Username',
  'elements.fields.username.placeholder': 'Enter your username',
  'elements.fields.password.label': 'Password',
  'elements.fields.password.placeholder': 'Enter your password',
  'elements.fields.first_name.label': 'First Name',
  'elements.fields.first_name.placeholder': 'Enter your first name',
  'elements.fields.last_name.label': 'Last Name',
  'elements.fields.last_name.placeholder': 'Enter your last name',
  'elements.fields.email.label': 'Email',
  'elements.fields.email.placeholder': 'Enter your email',
  'elements.fields.organization.name.label': 'Organization Name',
  'elements.fields.organization.handle.label': 'Organization Handle',
  'elements.fields.organization.description.label': 'Organization Description',
  'elements.fields.organization.select.label': 'Select Organization',
  'elements.fields.organization.select.placeholder': 'Choose an organization',

  /* Validation */
  'validations.required.field.error': 'This field is required',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'Sign In',
  'signin.subheading': 'Welcome back! Please sign in to continue.',

  /* Base Sign Up */
  'signup.heading': 'Sign Up',
  'signup.subheading': 'Create a new account to get started.',

  /* Email OTP */
  'email.otp.heading': 'OTP Verification',
  'email.otp.subheading': 'Enter the code sent to your email address.',
  'email.otp.buttons.submit.text': 'Continue',

  /* Identifier First */
  'identifier.first.heading': 'Sign In',
  'identifier.first.subheading': 'Enter your username or email address.',
  'identifier.first.buttons.submit.text': 'Continue',

  /* SMS OTP */
  'sms.otp.heading': 'OTP Verification',
  'sms.otp.subheading': 'Enter the code sent to your phone number.',
  'sms.otp.buttons.submit.text': 'Continue',

  /* TOTP */
  'totp.heading': 'Verify Your Identity',
  'totp.subheading': 'Enter the code from your authenticator app.',
  'totp.buttons.submit.text': 'Continue',

  /* Username Password */
  'username.password.heading': 'Sign In',
  'username.password.subheading': 'Enter your username and password to continue.',
  'username.password.buttons.submit.text': 'Continue',

  /* Passkeys */
  'passkey.button.use': 'Sign in with Passkey',
  'passkey.signin.heading': 'Sign in with Passkey',
  'passkey.register.heading': 'Register Passkey',
  'passkey.register.description': 'Create a passkey to securely sign in to your account without a password.',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'Profile',
  'user.profile.update.generic.error': 'An error occurred while updating your profile. Please try again.',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': 'Switch Organization',
  'organization.switcher.loading.placeholder.organizations': 'Loading organizations...',
  'organization.switcher.members': 'members',
  'organization.switcher.member': 'member',
  'organization.switcher.create.organization': 'Create Organization',
  'organization.switcher.manage.organizations': 'Manage Organizations',
  'organization.switcher.buttons.manage.text': 'Manage',
  'organization.switcher.organizations.heading': 'Organizations',
  'organization.switcher.buttons.switch.text': 'Switch',
  'organization.switcher.no.access': 'No Access',
  'organization.switcher.status.label': 'Status:',
  'organization.switcher.showing.count': 'Showing {showing} of {total} organizations',
  'organization.switcher.buttons.refresh.text': 'Refresh',
  'organization.switcher.buttons.load_more.text': 'Load More Organizations',
  'organization.switcher.loading.more': 'Loading...',
  'organization.switcher.no.organizations': 'No organizations found',
  'organization.switcher.error.prefix': 'Error:',

  'organization.profile.heading': 'Organization Profile',
  'organization.profile.loading': 'Loading organization...',
  'organization.profile.error': 'Failed to load organization',

  'organization.create.heading': 'Create Organization',
  'organization.create.buttons.create_organization.text': 'Create Organization',
  'organization.create.buttons.create_organization.loading.text': 'Creating...',
  'organization.create.buttons.cancel.text': 'Cancel',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': 'Loading...',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'Error',
  'errors.signin.components.not.available': 'Sign-in form is not available at the moment. Please try again later.',
  'errors.signin.initialization': 'An error occurred while initializing. Please try again later.',
  'errors.signin.flow.failure': 'An error occurred during the sign-in flow. Please try again later.',
  'errors.signin.flow.completion.failure':
    'An error occurred while completing the sign-in flow. Please try again later.',
  'errors.signin.flow.passkeys.failure': 'An error occurred while signing in with passkeys. Please try again later.',
  'errors.signin.flow.passkeys.completion.failure':
    'An error occurred while completing the passkeys sign-in flow. Please try again later.',
  'errors.signin.timeout': 'Time allowed to complete the step has expired.',

  'errors.signup.initialization': 'An error occurred while initializing. Please try again later.',
  'errors.signup.flow.failure': 'An error occurred during the sign-up flow. Please try again later.',
  'errors.signup.flow.initialization.failure':
    'An error occurred while initializing the sign-up flow. Please try again later.',
  'errors.signup.components.not.available': 'Sign-up form is not available at the moment. Please try again later.',
};

const metadata: I18nMetadata = {
  localeCode: 'en-US',
  countryCode: 'US',
  languageCode: 'en',
  displayName: 'English (United States)',
  direction: 'ltr',
};

const en_US: I18nBundle = {
  metadata,
  translations,
};

export default en_US;
