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
  'elements.buttons.signin.text': 'సైన్ ఇన్ చేయండి',
  'elements.buttons.signout.text': 'సైన్ అవుట్ చేయండి',
  'elements.buttons.signup.text': 'సైన్ అప్ చేయండి',
  'elements.buttons.submit.text': 'కొనసాగించండి',
  'elements.buttons.facebook.text': 'Facebook తో కొనసాగించండి',
  'elements.buttons.google.text': 'Google తో కొనసాగించండి',
  'elements.buttons.github.text': 'GitHub తో కొనసాగించండి',
  'elements.buttons.microsoft.text': 'Microsoft తో కొనసాగించండి',
  'elements.buttons.linkedin.text': 'LinkedIn తో కొనసాగించండి',
  'elements.buttons.ethereum.text': 'Ethereum తో సైన్ ఇన్ చేయండి',
  'elements.buttons.smsotp.text': 'SMS తో కొనసాగించండి',
  'elements.buttons.multi.option.text': '{connection} తో కొనసాగించండి',
  'elements.buttons.social.text': '{connection} తో కొనసాగించండి',

  /* Display */
  'elements.display.divider.or_separator': 'లేదా',
  'elements.display.copyable_text.copy': 'కాపీ చేయండి',
  'elements.display.copyable_text.copied': 'కాపీ చేయబడింది!',

  /* Fields */
  'elements.fields.generic.placeholder': 'మీ {field} ను నమోదు చేయండి',
  'elements.fields.username.label': 'వినియోగదారు పేరు',
  'elements.fields.username.placeholder': 'వినియోగదారు పేరును నమోదు చేయండి',
  'elements.fields.password.label': 'పాస్వర్డ్',
  'elements.fields.password.placeholder': 'పాస్వర్డ్ నమోదు చేయండి',
  'elements.fields.first_name.label': 'మొదటి పేరు',
  'elements.fields.first_name.placeholder': 'మీ మొదటి పేరును నమోదు చేయండి',
  'elements.fields.last_name.label': 'చివరి పేరు',
  'elements.fields.last_name.placeholder': 'మీ చివరి పేరును నమోదు చేయండి',
  'elements.fields.email.label': 'ఇమెయిల్',
  'elements.fields.email.placeholder': 'మీ ఇమెయిల్‌ను నమోదు చేయండి',
  'elements.fields.organization.name.label': 'సంస్థ పేరు',
  'elements.fields.organization.handle.label': 'సంస్థ హ్యాండిల్',
  'elements.fields.organization.description.label': 'సంస్థ వివరణ',
  'elements.fields.organization.select.label': 'ఆర్గనైజేషన్‌ను ఎంచుకోండి',
  'elements.fields.organization.select.placeholder': 'సంస్థను ఎంచుకోండి',

  /* Validation */
  'validations.required.field.error': 'ఈ ఫీల్డ్ అవసరం',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'సైన్ ఇన్ చేయండి',
  'signin.subheading': 'కొనసాగించడానికి మీ వివరాలు ఇవ్వండి.',

  /* Base Sign Up */
  'signup.heading': 'సైన్ అప్ చేయండి',
  'signup.subheading': 'కొత్త అకౌంట్ సృష్టించండి.',

  /* Email OTP */
  'email.otp.heading': 'OTP వెరిఫికేషన్',
  'email.otp.subheading': 'మీ ఇమెయిల్‌కి పంపిన కోడ్‌ను నమోదు చేయండి.',
  'email.otp.buttons.submit.text': 'కొనసాగించండి',

  /* Identifier First */
  'identifier.first.heading': 'సైన్ ఇన్ చేయండి',
  'identifier.first.subheading': 'మీ యూజర్ పేరు లేదా ఇమెయిల్ ఇవ్వండి.',
  'identifier.first.buttons.submit.text': 'కొనసాగించండి',

  /* SMS OTP */
  'sms.otp.heading': 'OTP వెరిఫికేషన్',
  'sms.otp.subheading': 'మీ ఫోన్ నంబర్‌కి పంపిన కోడ్‌ను నమోదు చేయండి.',
  'sms.otp.buttons.submit.text': 'కొనసాగించండి',

  /* TOTP */
  'totp.heading': 'మీ గుర్తింపును ధృవీకరించండి',
  'totp.subheading': 'మీ ఆథెంటికేటర్ యాప్‌లోని కోడ్‌ను నమోదు చేయండి.',
  'totp.buttons.submit.text': 'కొనసాగించండి',

  /* Username Password */
  'username.password.buttons.submit.text': 'కొనసాగించండి',
  'username.password.heading': 'సైన్ ఇన్ చేయండి',
  'username.password.subheading': 'మీ యూజర్ పేరు మరియు పాస్‌వర్డ్ ఇవ్వండి.',

  /* Passkeys */
  'passkey.button.use': 'Passkey తో సైన్ ఇన్ చేయండి',
  'passkey.signin.heading': 'Passkey తో సైన్ ఇన్ చేయండి',
  'passkey.register.heading': 'Passkey ని నమోదు చేయండి',
  'passkey.register.description':
    'పాస్‌వర్డ్ లేకుండా మీ ఖాతాలోకి సురక్షితంగా సైన్ ఇన్ చేయడానికి Passkey ని సృష్టించండి.',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'ప్రొఫైల్',
  'user.profile.update.generic.error': 'ప్రొఫైల్ అప్‌డేట్ చేస్తూ లోపం వచ్చింది. దయచేసి మళ్లీ ప్రయత్నించండి.',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': 'ఆర్గనైజేషన్ మార్చండి',
  'organization.switcher.loading.placeholder.organizations': 'ఆర్గనైజేషన్‌లు లోడ్ అవుతున్నాయి...',
  'organization.switcher.members': 'సభ్యులు',
  'organization.switcher.member': 'సభ్యుడు',
  'organization.switcher.create.organization': 'ఆర్గనైజేషన్ సృష్టించండి',
  'organization.switcher.manage.organizations': 'ఆర్గనైజేషన్‌లను నిర్వహించండి',
  'organization.switcher.buttons.manage.text': 'నిర్వహించండి',
  'organization.switcher.organizations.heading': 'ఆర్గనైజేషన్‌లు',
  'organization.switcher.buttons.switch.text': 'మార్చండి',
  'organization.switcher.no.access': 'యాక్సెస్ లేదు',
  'organization.switcher.status.label': 'స్టేటస్:',
  'organization.switcher.showing.count': '{total} లో {showing} ఆర్గనైజేషన్‌లు చూపుతున్నాయి',
  'organization.switcher.buttons.refresh.text': 'రిఫ్రెష్ చేయండి',
  'organization.switcher.buttons.load_more.text': 'మరిన్ని ఆర్గనైజేషన్‌లను లోడ్ చేయండి',
  'organization.switcher.loading.more': 'లోడ్ అవుతోంది...',
  'organization.switcher.no.organizations': 'ఏ ఆర్గనైజేషన్‌లు లభించలేదు',
  'organization.switcher.error.prefix': 'లోపం:',

  'organization.profile.heading': 'ఆర్గనైజేషన్ ప్రొఫైల్',
  'organization.profile.loading': 'లోడ్ అవుతోంది...',
  'organization.profile.error': 'ఆర్గనైజేషన్‌ను లోడ్ చేయడం విఫలమైంది',

  'organization.create.heading': 'ఆర్గనైజేషన్ సృష్టించండి',
  'organization.create.buttons.create_organization.text': 'సృష్టించండి',
  'organization.create.buttons.create_organization.loading.text': 'సృష్టిస్తోంది...',
  'organization.create.buttons.cancel.text': 'రద్దు చేయండి',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': 'లోడ్ అవుతోంది...',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'లోపం',
  'errors.signin.initialization': 'ప్రారంభించేటప్పుడు లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.flow.failure': 'సైన్ ఇన్ ప్రక్రియలో లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.flow.completion.failure': 'సైన్ ఇన్ పూర్తి చేయడంలో లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.flow.passkeys.failure': 'పాస్‌కీలతో సైన్ ఇన్ చేస్తూ లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.flow.passkeys.completion.failure':
    'పాస్‌కీ సైన్ ఇన్ పూర్తి చేయడంలో లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signup.initialization': 'ప్రారంభించేటప్పుడు లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signup.flow.failure': 'సైన్ అప్ ప్రక్రియలో లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signup.flow.initialization.failure':
    'సైన్ అప్ ప్రక్రియను ప్రారంభించేటప్పుడు లోపం వచ్చింది. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signup.components.not.available':
    'సైన్ అప్ ఫారం ప్రస్తుతం అందుబాటులో లేదు. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.components.not.available':
    'సైన్ ఇన్ ఫారం ప్రస్తుతం అందుబాటులో లేదు. దయచేసి తరువాత మళ్లీ ప్రయత్నించండి.',
  'errors.signin.timeout': 'దశను పూర్తి చేయడానికి అనుమతించబడిన సమయం ముగిసింది.',
};

const metadata: I18nMetadata = {
  localeCode: 'te-IN',
  countryCode: 'IN',
  languageCode: 'te',
  displayName: 'తెలుగు (భారతదేశం)',
  direction: 'ltr',
};

const te_IN: I18nBundle = {
  metadata,
  translations,
};

export default te_IN;
