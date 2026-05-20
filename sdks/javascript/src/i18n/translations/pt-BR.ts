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
  'elements.buttons.signin.text': 'Entrar',
  'elements.buttons.signout.text': 'Sair',
  'elements.buttons.signup.text': 'Cadastre-se',
  'elements.buttons.submit.text': 'Continuar',
  'elements.buttons.facebook.text': 'Entrar com Facebook',
  'elements.buttons.google.text': 'Entrar com Google',
  'elements.buttons.github.text': 'Entrar com GitHub',
  'elements.buttons.microsoft.text': 'Entrar com Microsoft',
  'elements.buttons.linkedin.text': 'Entrar com LinkedIn',
  'elements.buttons.ethereum.text': 'Entrar com Ethereum',
  'elements.buttons.smsotp.text': 'Entrar com SMS',
  'elements.buttons.multi.option.text': 'Entrar com {connection}',
  'elements.buttons.social.text': 'Entrar com {connection}',

  /* Display */
  'elements.display.divider.or_separator': 'OU',
  'elements.display.copyable_text.copy': 'Cópia',
  'elements.display.copyable_text.copied': 'Copiado!',

  /* Fields */
  'elements.fields.generic.placeholder': 'Digite seu {field}',
  'elements.fields.username.label': 'Nome de usuário',
  'elements.fields.username.placeholder': 'Digite o nome de usuário',
  'elements.fields.password.label': 'Senha',
  'elements.fields.password.placeholder': 'Digite sua senha',
  'elements.fields.first_name.label': 'Primeiro nome',
  'elements.fields.first_name.placeholder': 'Digite seu primeiro nome',
  'elements.fields.last_name.label': 'Sobrenome',
  'elements.fields.last_name.placeholder': 'Digite seu sobrenome',
  'elements.fields.email.label': 'Email',
  'elements.fields.email.placeholder': 'Digite seu email',
  'elements.fields.organization.name.label': 'Nome da Organização',
  'elements.fields.organization.handle.label': 'Identificador da Organização',
  'elements.fields.organization.description.label': 'Descrição da Organização',
  'elements.fields.organization.select.label': 'Selecionar Organização',
  'elements.fields.organization.select.placeholder': 'Selecione uma organização',

  /* Validation */
  'validations.required.field.error': 'Este campo é obrigatório',

  /* |---------------------------------------------------------------| */
  /* |                        Widgets                                | */
  /* |---------------------------------------------------------------| */

  /* Base Sign In */
  'signin.heading': 'Entrar',
  'signin.subheading': 'Digite suas credencias para continuar.',

  /* Base Sign Up */
  'signup.heading': 'Cadastra-se',
  'signup.subheading': 'Crie uma nova conta para iniciar.',

  /* Email OTP */
  'email.otp.heading': 'Verificação OTP',
  'email.otp.subheading': 'Digite o código enviado para seu e-mail.',
  'email.otp.buttons.submit.text': 'Continue',

  /* Identifier First */
  'identifier.first.heading': 'Entrar',
  'identifier.first.subheading': 'Digite seu usuário ou e-mail.',
  'identifier.first.buttons.submit.text': 'Continue',

  /* SMS OTP */
  'sms.otp.heading': 'Verificação OTP',
  'sms.otp.subheading': 'Digite o código enviado para seu telefone.',
  'sms.otp.buttons.submit.text': 'Continue',

  /* TOTP */
  'totp.heading': 'Verifique sua identidade',
  'totp.subheading': 'Digite o código do seu aplicativo autenticador.',
  'totp.buttons.submit.text': 'Continue',

  /* Username Password */
  'username.password.buttons.submit.text': 'Continue',
  'username.password.heading': 'Entrar',
  'username.password.subheading': 'Digite seu usuário e senha para continuar.',

  /* Passkeys */
  'passkey.button.use': 'Entrar com Passkey',
  'passkey.signin.heading': 'Entrar com Passkey',
  'passkey.register.heading': 'Registrar Passkey',
  'passkey.register.description': 'Crie uma passkey para entrar em sua conta com segurança sem uma senha.',

  /* |---------------------------------------------------------------| */
  /* |                          User Profile                         | */
  /* |---------------------------------------------------------------| */

  'user.profile.heading': 'Perfil',
  'user.profile.update.generic.error': 'Ocorreu um erro ao atualizar seu perfil. Tente novamente.',

  /* |---------------------------------------------------------------| */
  /* |                     Organization Switcher                     | */
  /* |---------------------------------------------------------------| */

  'organization.switcher.switch.organization': 'Trocar Organização',
  'organization.switcher.loading.placeholder.organizations': 'Carregando organizações...',
  'organization.switcher.members': 'membros',
  'organization.switcher.member': 'membro',
  'organization.switcher.create.organization': 'Criar Organização',
  'organization.switcher.manage.organizations': 'Gerenciar Organizações',
  'organization.switcher.buttons.manage.text': 'Gerenciar',
  'organization.switcher.organizations.heading': 'Organizações',
  'organization.switcher.buttons.switch.text': 'Trocar',
  'organization.switcher.no.access': 'Sem Acesso',
  'organization.switcher.status.label': 'Situação:',
  'organization.switcher.showing.count': 'Exibindo {showing} de {total} organizações',
  'organization.switcher.buttons.refresh.text': 'Atualizar',
  'organization.switcher.buttons.load_more.text': 'Carregar Mais Organizações',
  'organization.switcher.loading.more': 'Carregando...',
  'organization.switcher.no.organizations': 'Nenhuma organização encontrada',
  'organization.switcher.error.prefix': 'Erro:',

  'organization.profile.heading': 'Perfil da Organização',
  'organization.profile.loading': 'Carregando organização...',
  'organization.profile.error': 'Falha ao carregar organização',

  'organization.create.heading': 'Criar Organização',
  'organization.create.buttons.create_organization.text': 'Criar Organização',
  'organization.create.buttons.create_organization.loading.text': 'Criando...',
  'organization.create.buttons.cancel.text': 'Cancelar',

  /* |---------------------------------------------------------------| */
  /* |                        Messages                               | */
  /* |---------------------------------------------------------------| */

  'messages.loading.placeholder': 'Carregando...',

  /* |---------------------------------------------------------------| */
  /* |                        Errors                                 | */
  /* |---------------------------------------------------------------| */

  'errors.heading': 'Erro',
  'errors.signin.initialization': 'Ocorreu um erro ao inicializar. Tente novamente mais tarde.',
  'errors.signin.flow.failure': 'Ocorreu um erro durante o login. Tente novamente mais tarde.',
  'errors.signin.flow.completion.failure': 'Ocorreu um erro ao completar o login. Tente novamente mais tarde.',
  'errors.signin.flow.passkeys.failure':
    'Ocorreu um erro ao entrar com as chaves de acesso (passkeys). Tente novamente mais tarde.',
  'errors.signin.flow.passkeys.completion.failure':
    'Ocorreu um erro ao completar o login com as chaves de acesso (passkeys). Tente novamente mais tarde.',
  'errors.signup.initialization': 'Ocorreu um erro durante a inicialização. Tente novamente mais tarde.',
  'errors.signup.flow.failure': 'Ocorreu um erro durante o fluxo de cadastro. Tente novamente mais tarde.',
  'errors.signup.flow.initialization.failure':
    'Ocorreu um erro ao inicializar o fluxo de cadastro. Tente novamente mais tarde.',
  'errors.signup.components.not.available':
    'O formulário de cadastro não está disponível no momento. Tente novamente mais tarde.',
  'errors.signin.components.not.available':
    'O formulário de login não está disponível no momento. Tente novamente mais tarde.',
  'errors.signin.timeout': 'O tempo permitido para concluir a etapa expirou.',
};

const metadata: I18nMetadata = {
  localeCode: 'pt-BR',
  countryCode: 'BR',
  languageCode: 'pt',
  displayName: 'Português (Brazil)',
  direction: 'ltr',
};

const pt_BR: I18nBundle = {
  metadata,
  translations,
};

export default pt_BR;
