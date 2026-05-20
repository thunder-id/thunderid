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
  'elements.buttons.signin.text': 'साइन इन',
  'elements.buttons.signout.text': 'साइन आउट',
  'elements.buttons.signup.text': 'साइन अप',
  'elements.buttons.submit.text': 'जारी रखें',
  'elements.buttons.facebook.text': 'Facebook के साथ जारी रखें',
  'elements.buttons.google.text': 'Google के साथ जारी रखें',
  'elements.buttons.github.text': 'GitHub के साथ जारी रखें',
  'elements.buttons.microsoft.text': 'Microsoft के साथ जारी रखें',
  'elements.buttons.linkedin.text': 'LinkedIn के साथ जारी रखें',
  'elements.buttons.ethereum.text': 'Ethereum के साथ साइन इन करें',
  'elements.buttons.smsotp.text': 'SMS के साथ जारी रखें',
  'elements.buttons.multi.option.text': '{connection} के साथ जारी रखें',
  'elements.buttons.social.text': '{connection} के साथ जारी रखें',

  /* Display */
  'elements.display.divider.or_separator': 'या',
  'elements.display.copyable_text.copy': 'प्रतिलिपि',
  'elements.display.copyable_text.copied': 'नकल की गई!',

  /* Fields */
  'elements.fields.generic.placeholder': '{field} दर्ज करें',
  'elements.fields.username.label': 'उपयोगकर्ता नाम',
  'elements.fields.username.placeholder': 'अपना उपयोगकर्ता नाम दर्ज करें',
  'elements.fields.password.label': 'पासवर्ड',
  'elements.fields.password.placeholder': 'अपना पासवर्ड दर्ज करें',
  'elements.fields.first_name.label': 'पहला नाम',
  'elements.fields.first_name.placeholder': 'अपना पहला नाम दर्ज करें',
  'elements.fields.last_name.label': 'अंतिम नाम',
  'elements.fields.last_name.placeholder': 'अपना अंतिम नाम दर्ज करें',
  'elements.fields.email.label': 'ईमेल',
  'elements.fields.email.placeholder': 'अपना ईमेल दर्ज करें',
  'elements.fields.organization.name.label': 'संगठन का नाम',
  'elements.fields.organization.handle.label': 'संगठन हैंडल',
  'elements.fields.organization.description.label': 'संगठन विवरण',
  'elements.fields.organization.select.label': 'संगठन चुनें',
  'elements.fields.organization.select.placeholder': 'एक संगठन चुनें',

  /* Validation */
  'validations.required.field.error': 'यह फील्ड आवश्यक है',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'साइन इन',
  'signin.subheading': 'जारी रखने के लिए अपनी प्रमाणिक जानकारी दर्ज करें।',

  /* Base Sign Up */
  'signup.heading': 'साइन अप',
  'signup.subheading': 'शुरू करने के लिए नया खाता बनाएं।',

  /* Email OTP */
  'email.otp.heading': 'OTP सत्यापन',
  'email.otp.subheading': 'अपनी ईमेल पर भेजा गया कोड दर्ज करें।',
  'email.otp.buttons.submit.text': 'जारी रखें',

  /* Identifier First */
  'identifier.first.heading': 'साइन इन',
  'identifier.first.subheading': 'अपना उपयोगकर्ता नाम या ईमेल दर्ज करें।',
  'identifier.first.buttons.submit.text': 'जारी रखें',

  /* SMS OTP */
  'sms.otp.heading': 'OTP सत्यापन',
  'sms.otp.subheading': 'अपने फ़ोन नंबर पर भेजा गया कोड दर्ज करें।',
  'sms.otp.buttons.submit.text': 'जारी रखें',

  /* TOTP */
  'totp.heading': 'अपनी पहचान सत्यापित करें',
  'totp.subheading': 'अपने ऑथेंटिकेटर ऐप से कोड दर्ज करें।',
  'totp.buttons.submit.text': 'जारी रखें',

  /* Username Password */
  'username.password.buttons.submit.text': 'जारी रखें',
  'username.password.heading': 'साइन इन',
  'username.password.subheading': 'अपना उपयोगकर्ता नाम और पासवर्ड दर्ज करें।',

  /* Passkeys */
  'passkey.button.use': 'Passkey के साथ साइन इन करें',
  'passkey.signin.heading': 'Passkey के साथ साइन इन करें',
  'passkey.register.heading': 'Passkey पंजीकृत करें',
  'passkey.register.description': 'बिना पासवर्ड के अपने खाते में सुरक्षित रूप से साइन इन करने के लिए एक Passkey बनाएं।',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'प्रोफ़ाइल',
  'user.profile.update.generic.error': 'प्रोफ़ाइल अपडेट करते समय त्रुटि हुई। कृपया पुनः प्रयास करें।',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': 'संगठन बदलें',
  'organization.switcher.loading.placeholder.organizations': 'संगठन लोड हो रहे हैं...',
  'organization.switcher.members': 'सदस्य',
  'organization.switcher.member': 'सदस्य',
  'organization.switcher.create.organization': 'संगठन बनाएं',
  'organization.switcher.manage.organizations': 'संगठनों का प्रबंधन करें',
  'organization.switcher.buttons.manage.text': 'प्रबंधित करें',
  'organization.switcher.organizations.heading': 'संगठन',
  'organization.switcher.buttons.switch.text': 'बदलें',
  'organization.switcher.no.access': 'कोई पहुँच नहीं',
  'organization.switcher.status.label': 'स्थिति:',
  'organization.switcher.showing.count': '{total} में से {showing} संगठन दिखा रहे हैं',
  'organization.switcher.buttons.refresh.text': 'रिफ्रेश',
  'organization.switcher.buttons.load_more.text': 'और संगठन लोड करें',
  'organization.switcher.loading.more': 'लोड हो रहा है...',
  'organization.switcher.no.organizations': 'कोई संगठन नहीं मिला',
  'organization.switcher.error.prefix': 'त्रुटि:',

  'organization.profile.heading': 'संगठन प्रोफ़ाइल',
  'organization.profile.loading': 'संगठन लोड हो रहा है...',
  'organization.profile.error': 'संगठन लोड करने में विफल',

  'organization.create.heading': 'संगठन बनाएं',
  'organization.create.buttons.create_organization.text': 'संगठन बनाएं',
  'organization.create.buttons.create_organization.loading.text': 'बनाया जा रहा है...',
  'organization.create.buttons.cancel.text': 'रद्द करें',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': 'लोड हो रहा है...',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'त्रुटि',
  'errors.signin.initialization': 'इनिशियलाइज़ेशन में त्रुटि हुई। कृपया बाद में पुनः प्रयास करें।',
  'errors.signin.flow.failure': 'साइन-इन प्रक्रिया में त्रुटि। कृपया बाद में पुनः प्रयास करें।',
  'errors.signin.flow.completion.failure': 'साइन-इन प्रक्रिया पूरी करते समय त्रुटि। कृपया बाद में पुनः प्रयास करें।',
  'errors.signin.flow.passkeys.failure': 'पासकीज़ के साथ साइन-इन करते समय त्रुटि।',
  'errors.signin.flow.passkeys.completion.failure': 'पासकीज़ साइन-इन पूरी करते समय त्रुटि।',
  'errors.signup.initialization': 'प्रारंभीकरण के दौरान एक त्रुटि हुई। कृपया बाद में पुनः प्रयास करें।',
  'errors.signup.flow.failure': 'साइन-अप प्रक्रिया में त्रुटि। कृपया बाद में पुनः प्रयास करें।',
  'errors.signup.flow.initialization.failure':
    'साइन-अप प्रक्रिया प्रारंभ करते समय त्रुटि। कृपया बाद में पुनः प्रयास करें।',
  'errors.signup.components.not.available': 'साइन-अप फॉर्म फिलहाल उपलब्ध नहीं है। कृपया बाद में पुनः प्रयास करें।',
  'errors.signin.components.not.available': 'साइन-इन फॉर्म फिलहाल उपलब्ध नहीं है। कृपया बाद में पुनः प्रयास करें।',
  'errors.signin.timeout': 'स्टेप पूरा करने के लिए दिया गया समय खत्म हो गया है।',
};

const metadata: I18nMetadata = {
  localeCode: 'hi-IN',
  countryCode: 'IN',
  languageCode: 'hi',
  displayName: 'हिन्दी (भारत)',
  direction: 'ltr',
};

const hi_IN: I18nBundle = {
  metadata,
  translations,
};

export default hi_IN;
