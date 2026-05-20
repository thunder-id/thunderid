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
  'elements.buttons.signin.text': 'ログイン',
  'elements.buttons.signout.text': 'ログアウト',
  'elements.buttons.signup.text': 'サインアップ',
  'elements.buttons.submit.text': '続行',
  'elements.buttons.facebook.text': 'Facebookで続行',
  'elements.buttons.google.text': 'Googleで続行',
  'elements.buttons.github.text': 'GitHubで続行',
  'elements.buttons.microsoft.text': 'Microsoftで続行',
  'elements.buttons.linkedin.text': 'LinkedInで続行',
  'elements.buttons.ethereum.text': 'Ethereumでサインイン',
  'elements.buttons.smsotp.text': 'SMSで続行',
  'elements.buttons.multi.option.text': '{connection}で続行',
  'elements.buttons.social.text': '{connection}で続行',

  /* Display */
  'elements.display.divider.or_separator': 'または',
  'elements.display.copyable_text.copy': 'コピー',
  'elements.display.copyable_text.copied': 'コピーしました！',

  /* Fields */
  'elements.fields.generic.placeholder': '{field}を入力してください',
  'elements.fields.username.label': 'ユーザー名',
  'elements.fields.username.placeholder': 'ユーザー名を入力してください',
  'elements.fields.password.label': 'パスワード',
  'elements.fields.password.placeholder': 'パスワードを入力してください',
  'elements.fields.first_name.label': '名',
  'elements.fields.first_name.placeholder': '名を入力してください',
  'elements.fields.last_name.label': '姓',
  'elements.fields.last_name.placeholder': '姓を入力してください',
  'elements.fields.email.label': 'メールアドレス',
  'elements.fields.email.placeholder': 'メールアドレスを入力してください',
  'elements.fields.organization.name.label': '組織名',
  'elements.fields.organization.handle.label': '組織ハンドル',
  'elements.fields.organization.description.label': '組織の説明',
  'elements.fields.organization.select.label': '組織を選択',
  'elements.fields.organization.select.placeholder': '組織を選択してください',

  /* Validation */
  'validations.required.field.error': 'この項目は必須です',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'ログイン',
  'signin.subheading': '続行するには認証情報を入力してください。',

  /* Base Sign Up */
  'signup.heading': 'サインアップ',
  'signup.subheading': 'はじめるには新しいアカウントを作成してください。',

  /* Email OTP */
  'email.otp.heading': 'OTP認証',
  'email.otp.subheading': 'メールに送信されたコードを入力してください。',
  'email.otp.buttons.submit.text': '続行',

  /* Identifier First */
  'identifier.first.heading': 'ログイン',
  'identifier.first.subheading': 'ユーザー名またはメールアドレスを入力してください。',
  'identifier.first.buttons.submit.text': '続行',

  /* SMS OTP */
  'sms.otp.heading': 'OTP認証',
  'sms.otp.subheading': '電話番号に送信されたコードを入力してください。',
  'sms.otp.buttons.submit.text': '続行',

  /* TOTP */
  'totp.heading': '本人確認',
  'totp.subheading': '認証アプリのコードを入力してください。',
  'totp.buttons.submit.text': '続行',

  /* Username Password */
  'username.password.buttons.submit.text': '続行',
  'username.password.heading': 'ログイン',
  'username.password.subheading': 'ユーザー名とパスワードを入力してください。',

  /* Passkeys */
  'passkey.button.use': 'パスキーでサインイン',
  'passkey.signin.heading': 'パスキーでサインイン',
  'passkey.register.heading': 'パスキーを登録',
  'passkey.register.description': 'パスワードなしでアカウントに安全にサインインするためのパスキーを作成します。',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'プロフィール',
  'user.profile.update.generic.error': 'プロフィール更新中にエラーが発生しました。もう一度お試しください。',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': '組織を切り替え',
  'organization.switcher.loading.placeholder.organizations': '組織を読み込み中…',
  'organization.switcher.members': 'メンバー',
  'organization.switcher.member': 'メンバー',
  'organization.switcher.create.organization': '組織を作成',
  'organization.switcher.manage.organizations': '組織を管理',
  'organization.switcher.buttons.manage.text': '管理',
  'organization.switcher.organizations.heading': '組織',
  'organization.switcher.buttons.switch.text': '切り替え',
  'organization.switcher.no.access': 'アクセス権がありません',
  'organization.switcher.status.label': 'ステータス:',
  'organization.switcher.showing.count': '全{total}件中{showing}件を表示',
  'organization.switcher.buttons.refresh.text': '更新',
  'organization.switcher.buttons.load_more.text': 'さらに読み込む',
  'organization.switcher.loading.more': '読み込み中…',
  'organization.switcher.no.organizations': '組織が見つかりません',
  'organization.switcher.error.prefix': 'エラー:',

  'organization.profile.heading': '組織プロファイル',
  'organization.profile.loading': '組織を読み込み中…',
  'organization.profile.error': '組織の読み込みに失敗しました',

  'organization.create.heading': '組織の作成',
  'organization.create.buttons.create_organization.text': '組織を作成',
  'organization.create.buttons.create_organization.loading.text': '作成中…',
  'organization.create.buttons.cancel.text': 'キャンセル',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': '読み込み中…',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'エラー',
  'errors.signin.initialization': '初期化中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signin.flow.failure': 'サインイン処理中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signin.flow.completion.failure': 'サインイン処理の完了中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signin.flow.passkeys.failure': 'パスキーでのサインイン中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signin.flow.passkeys.completion.failure':
    'パスキーによるサインイン完了中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signup.initialization': '初期化中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signup.flow.failure': 'サインアップフロー中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signup.flow.initialization.failure':
    'サインアップフローの初期化中にエラーが発生しました。後でもう一度お試しください。',
  'errors.signup.components.not.available': 'サインアップフォームは現在利用できません。後でもう一度お試しください。',
  'errors.signin.components.not.available': 'サインインフォームは現在利用できません。後でもう一度お試しください。',
  'errors.signin.timeout': 'ステップを完了するために許可された時間が経過しました。',
};

const metadata: I18nMetadata = {
  localeCode: 'ja-JP',
  countryCode: 'JP',
  languageCode: 'ja',
  displayName: '日本語 (日本)',
  direction: 'ltr',
};

const ja_JP: I18nBundle = {
  metadata,
  translations,
};

export default ja_JP;
