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
  'elements.buttons.signin.text': 'ලොග් වෙන්න',
  'elements.buttons.signout.text': 'ඉවත් වෙන්න',
  'elements.buttons.signup.text': 'ලියාපදිංචි වෙන්න',
  'elements.buttons.submit.text': 'ඉදිරියට යන්න',
  'elements.buttons.facebook.text': 'Facebook සමග ඉදිරියට යන්න',
  'elements.buttons.google.text': 'Google සමග ඉදිරියට යන්න',
  'elements.buttons.github.text': 'GitHub සමග ඉදිරියට යන්න',
  'elements.buttons.microsoft.text': 'Microsoft සමග ඉදිරියට යන්න',
  'elements.buttons.linkedin.text': 'LinkedIn සමග ඉදිරියට යන්න',
  'elements.buttons.ethereum.text': 'Ethereum සමග ඉදිරියට යන්න',
  'elements.buttons.smsotp.text': 'SMS සමග ඉදිරියට යන්න',
  'elements.buttons.multi.option.text': '{connection} සමග ඉදිරියට යන්න',
  'elements.buttons.social.text': '{connection} සමග ඉදිරියට යන්න',

  /* Display */
  'elements.display.divider.or_separator': 'හෝ',
  'elements.display.copyable_text.copy': 'පිටපත් කරන්න',
  'elements.display.copyable_text.copied': 'පිටපත් කළා!',

  /* Fields */
  'elements.fields.generic.placeholder': 'ඔබේ {field} ඇතුලත් කරන්න',
  'elements.fields.username.label': 'පරිශීලක නාමය',
  'elements.fields.username.placeholder': 'පරිශීලක නාමය ඇතුලත් කරන්න',
  'elements.fields.password.label': 'මුරපදය',
  'elements.fields.password.placeholder': 'මුරපදය ඇතුලත් කරන්න',
  'elements.fields.first_name.label': 'මුල් නම',
  'elements.fields.first_name.placeholder': 'ඔබේ මුල් නම ඇතුලත් කරන්න',
  'elements.fields.last_name.label': 'අවසන් නම',
  'elements.fields.last_name.placeholder': 'ඔබේ අවසන් නම ඇතුලත් කරන්න',
  'elements.fields.email.label': 'ඊමේල්',
  'elements.fields.email.placeholder': 'ඔබේ ඊමේල් ලිපිනය ඇතුලත් කරන්න',
  'elements.fields.organization.name.label': 'සංවිධානයේ නම',
  'elements.fields.organization.handle.label': 'සංවිධාන හැඩුනුම්පත',
  'elements.fields.organization.description.label': 'සංවිධානයේ විස්තරය',
  'elements.fields.organization.select.label': 'සංවිධානය තෝරන්න',
  'elements.fields.organization.select.placeholder': 'සංවිධානයක් සැළුම් කරන්න',

  /* Validation */
  'validations.required.field.error': 'මෙම ක්ෂේත්‍රය අවශ්‍යයි',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'ලොග් වෙන්න',
  'signin.subheading': 'ඉදිරියට යාමට ඔබේ සත්‍යාපන තොරතුරු ඇතුළත් කරන්න.',

  /* Base Sign Up */
  'signup.heading': 'ලියාපදිංචි වන්න',
  'signup.subheading': 'ආරම්භ කිරීමට නව ගිණුමක් සාදන්න.',

  /* Email OTP */
  'email.otp.heading': 'OTP සත්‍යාපනය',
  'email.otp.subheading': 'ඔබේ විද්‍යුත් තැපැල් ලිපිනයට යවන ලද කේතය ඇතුළත් කරන්න.',
  'email.otp.buttons.submit.text': 'ඉදිරියට යන්න',

  /* Identifier First */
  'identifier.first.heading': 'ලොග් වෙන්න',
  'identifier.first.subheading': 'ඔබේ පරිශීලක නාමය හෝ විද්‍යුත් තැපැල් ලිපිනය ඇතුළත් කරන්න.',
  'identifier.first.buttons.submit.text': 'ඉදිරියට යන්න',

  /* SMS OTP */
  'sms.otp.heading': 'OTP සත්‍යාපනය',
  'sms.otp.subheading': 'ඔබේ දුරකථන අංකයට යවන ලද කේතය ඇතුළත් කරන්න.',
  'sms.otp.buttons.submit.text': 'ඉදිරියට යන්න',

  /* TOTP */
  'totp.heading': 'ඔබගේ අනන්‍යතාවය තහවුරු කරන්න',
  'totp.subheading': 'ඔබේ authenticator යෙදුමෙන් ලබාගත් කේතය ඇතුළත් කරන්න.',
  'totp.buttons.submit.text': 'ඉදිරියට යන්න',

  /* Username Password */
  'username.password.buttons.submit.text': 'ඉදිරියට යන්න',
  'username.password.heading': 'ලොග් වෙන්න',
  'username.password.subheading': 'ඉදිරියට යාමට ඔබේ පරිශීලක නාමය සහ මුරපදය ඇතුළත් කරන්න.',

  /* Passkeys */
  'passkey.button.use': 'Passkey මගින් ඇතුල් වන්න',
  'passkey.signin.heading': 'Passkey මගින් ඇතුල් වන්න',
  'passkey.register.heading': 'Passkey ලියාපදිංචි කරන්න',
  'passkey.register.description': 'මුරපදයක් නොමැතිව ඔබේ ගිණුමට ආරක්ෂිතව ඇතුල් වීමට passkey එකක් සාදන්න.',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'පැතිකඩ',
  'user.profile.update.generic.error': 'ඔබේ පැතිකඩ යාවත්කාලීන කිරීමේදී දෝෂයක් ඇතිවිය.කරුණාකර නැවත උත්සාහ කරන්න',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': 'සංවිධානය මාරු කරන්න',
  'organization.switcher.loading.placeholder.organizations': 'සංවිධාන ලෝඩ් වෙමින්...',
  'organization.switcher.members': 'සාමාජිකයන්',
  'organization.switcher.member': 'සාමාජිකයා',
  'organization.switcher.create.organization': 'සංවිධානයක් සාදන්න',
  'organization.switcher.manage.organizations': 'සංවිධාන කළමනාකරණය කරන්න',
  'organization.switcher.buttons.manage.text': 'කළමනාකරණය කරන්න',
  'organization.switcher.organizations.heading': 'සංවිධාන',
  'organization.switcher.buttons.switch.text': 'මාරු කරන්න',
  'organization.switcher.no.access': 'ප්‍රවේශය නැත',
  'organization.switcher.status.label': 'තත්ත්වය:',
  'organization.switcher.showing.count': 'මුළු සංවිධාන {showing} න් {total} ක් පෙන්වමින්',
  'organization.switcher.buttons.refresh.text': 'නැවුම් කරන්න',
  'organization.switcher.buttons.load_more.text': 'තවත් සංවිධාන ලෝඩ් කරන්න',
  'organization.switcher.loading.more': 'ලෝඩ් වෙමින්...',
  'organization.switcher.no.organizations': 'සංවිධාන කිසිවක් හමු නොවීය.',
  'organization.switcher.error.prefix': 'දෝෂය:',

  'organization.profile.heading': 'සංවිධානයේ පැතිකඩ',
  'organization.profile.loading': 'සංවිධානය ලෝඩ් වෙමින්...',
  'organization.profile.error': 'සංවිධානය ලෝඩ් කිරීමට අසමත් විය',

  'organization.create.heading': 'සංවිධානය සාදන්න',
  'organization.create.buttons.create_organization.text': 'සංවිධානය සාදන්න',
  'organization.create.buttons.create_organization.loading.text': 'සාදමින්...',
  'organization.create.buttons.cancel.text': 'අවලංගු කරන්න',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': 'ලෝඩ් වෙමින්...',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'දෝෂය',
  'errors.signin.initialization': 'ආරම්භ කිරීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.flow.failure': 'ලොග් වීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.flow.completion.failure':
    'ලොග් වීමේ ක්‍රියාවලිය සම්පූර්ණ කිරීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.flow.passkeys.failure': 'passkeys සමඟ ලොග් වීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.flow.passkeys.completion.failure':
    'passkeys සමඟ ලොග් වීමේ ක්‍රියාවලිය සම්පූර්ණ කිරීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signup.initialization': 'ආරම්භ කිරීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signup.flow.failure': 'ගිණුම් තැනීමේ ක්‍රියාවලියේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signup.flow.initialization.failure':
    'ගිණුම් තැනීමේ ක්‍රියාවලිය ආරම්භ කිරීමේදී දෝෂයක් සිදු විය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signup.components.not.available': 'ගිණුම් තැනීමේ පිටුව දැන් ලබා ගත නොහැකිය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.components.not.available': 'ප්‍රවේශ වීමේ පිටුව දැන් ලබා ගත නොහැකිය. කරුණාකර පසුව නැවත උත්සාහ කරන්න.',
  'errors.signin.timeout': 'පියවර සම්පූර්ණ කිරීමට ලබා දී තිබූ කාලය ඉකුත් වී ඇත.',
};

const metadata: I18nMetadata = {
  localeCode: 'si_LK',
  countryCode: 'LK',
  languageCode: 'si',
  displayName: 'සිංහල (ශ්‍රී ලංකාව)',
  direction: 'ltr',
};

const si_LK: I18nBundle = {
  metadata,
  translations,
};

export default si_LK;
