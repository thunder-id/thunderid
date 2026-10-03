// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import { SequenceDiagram } from './SequenceDiagram';
import type { DocusaurusProductConfig } from '@site/docusaurus.product.config';

// Convention used across these diagrams:
//   - "Application" — user-facing OAuth client (i.e. when a User Agent is on stage).
//   - "Client" — pure protocol-layer caller (no User Agent on stage).
//   - the product name, read from the site config, names the authorization server.
//   - Gaps between adjacent actors are uniform within each diagram.

// useProductName reads the product's name from the site config, so a diagram never hardcodes the
// brand. Falls back to the site title, which is what Docusaurus guarantees is set.
function useProductName(): string {
  const { siteConfig } = useDocusaurusContext();
  return (
    (siteConfig.customFields?.product as DocusaurusProductConfig | undefined)?.project.name ?? siteConfig.title
  );
}

export function AuthorizationCodeDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['User', 'User Agent', 'Application', productName, 'Resource Server']}
      gaps={[280, 280, 280, 280]}
      ariaLabel={`Authorization Code grant flow: the user starts a sign-in, the application redirects the user agent to ${productName}, the user authenticates, ${productName} returns an authorization code via the redirect URI, the application exchanges the code for tokens, then calls a resource server with the access token.`}
      rows={[
        { from: 0, to: 2, label: 'Initiate sign-in' },
        { from: 2, to: 1, label: ['302 Redirect to', '/oauth2/authorize'] },
        { from: 1, to: 3, label: 'GET /oauth2/authorize', sublabel: ['response_type=code, client_id,', 'scope, code_challenge'] },
        { from: 3, to: 1, label: 'Sign-in page' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 3, label: 'POST credentials' },
        { from: 3, to: 1, label: ['302 Redirect with', '?code=...&state=...&iss=...'] },
        { from: 1, to: 2, label: 'Callback with code' },
        { from: 2, to: 3, label: 'POST /oauth2/token', sublabel: ['code, code_verifier,', 'client_auth'] },
        { from: 3, to: 2, label: ['200 OK — access token,', 'ID token, refresh token'] },
        { from: 2, to: 4, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 4, to: 2, label: '200 OK' },
      ]}
    />
  );
}

export function ClientCredentialsDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client', productName, 'Resource Server']}
      gaps={[340, 340]}
      ariaLabel="Client Credentials grant flow: the client authenticates with its credentials at the token endpoint, receives an access token bound to its own identity, then calls a resource server with the access token."
      rows={[
        { from: 0, to: 1, label: 'POST /oauth2/token', sublabel: ['grant_type=client_credentials,', 'scope, resource'] },
        { from: 1, to: 0, label: ['200 OK — access token', '(sub = client_id)'] },
        { from: 0, to: 2, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 2, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function OrganizationScopedClientCredentialsDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client', productName, 'Resource Server']}
      gaps={[340, 340]}
      ariaLabel={`Organization-scoped Client Credentials flow: the client posts to the organization-unit-prefixed token endpoint with one set of credentials, ${productName} resolves the named organization unit and checks the application may act for it, either because it owns the application or because a sharing policy reaches it, returns an access token issued for that unit, and the client calls a resource server with it.`}
      rows={[
        { from: 0, to: 1, label: 'POST /ou/{ouId}/oauth2/token', sublabel: ['grant_type=client_credentials,', 'one client ID and secret'] },
        { note: 'Resolve the named organization unit', between: [1, 1] },
        { note: 'Check the application may act for the OU as owner or through a sharing policy', between: [1, 1] },
        { from: 1, to: 0, label: ['200 OK — access token', 'issued for the named unit'] },
        { from: 0, to: 2, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 2, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function TokenExchangeDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client A', productName, 'Service B']}
      gaps={[360, 360]}
      ariaLabel={`Token Exchange flow: Client A holds a token, exchanges it at ${productName} for a new token scoped for Service B, then calls Service B.`}
      rows={[
        { from: 0, to: 1, label: 'POST /oauth2/token', sublabel: ['grant_type=token-exchange,', 'subject_token, audience'] },
        { from: 1, to: 0, label: ['200 OK — downscoped access token', '(aud = Service B)'] },
        { from: 0, to: 2, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <new_token>'] },
        { from: 2, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function PKCEDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Application', 'User Agent', productName, 'Resource Server']}
      gaps={[280, 280, 280]}
      ariaLabel={`PKCE flow: the application generates a code_verifier and code_challenge, sends the challenge with the authorization request, the user authenticates, then the application sends the verifier with the token request. ${productName} verifies the relationship before issuing tokens, and the application calls a resource server.`}
      rows={[
        { from: 0, to: 1, label: 'Redirect with code_challenge' },
        { from: 1, to: 2, label: 'GET /oauth2/authorize', sublabel: ['code_challenge,', 'code_challenge_method=S256'] },
        { from: 2, to: 1, label: '302 Redirect ?code=...' },
        { from: 1, to: 0, label: 'Callback with code' },
        { from: 0, to: 2, label: 'POST /oauth2/token', sublabel: 'code, code_verifier' },
        { from: 2, to: 0, label: '200 OK — tokens' },
        { from: 0, to: 3, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 3, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function PARDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['User', 'User Agent', 'Application', productName, 'Resource Server']}
      gaps={[280, 280, 280, 280]}
      ariaLabel={`Pushed Authorization Request flow: the user starts a sign-in, the application pushes authorization parameters to ${productName} over a back-channel, receives a request_uri, then redirects the user agent to the authorization endpoint with only the request_uri, completes the user sign-in, exchanges the code for tokens, and calls a resource server.`}
      rows={[
        { from: 0, to: 2, label: 'Initiate sign-in' },
        { from: 2, to: 3, label: 'POST /oauth2/par', sublabel: ['client_id, scope,', 'redirect_uri, code_challenge'] },
        { from: 3, to: 2, label: ['201 Created', 'request_uri, expires_in'] },
        { from: 2, to: 1, label: ['302 Redirect to /oauth2/authorize', '?request_uri=...'] },
        { from: 1, to: 3, label: 'GET /oauth2/authorize?request_uri=...' },
        { from: 3, to: 1, label: 'Sign-in page' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 3, label: 'POST credentials' },
        { from: 3, to: 1, label: '302 Redirect ?code=...' },
        { from: 1, to: 2, label: 'Callback with code' },
        { from: 2, to: 3, label: 'POST /oauth2/token' },
        { from: 3, to: 2, label: '200 OK — tokens' },
        { from: 2, to: 4, label: 'GET /resource', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 4, to: 2, label: '200 OK' },
      ]}
    />
  );
}

export function DPoPDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client', productName, 'Resource Server']}
      gaps={[380, 380]}
      ariaLabel={`DPoP flow: the client signs a DPoP proof with its keypair on every protected call. ${productName} binds the issued token to the proof's public key. The resource server verifies the binding on each request.`}
      rows={[
        { from: 0, to: 1, label: 'POST /oauth2/token', sublabel: ['DPoP: <proof JWT>', '(htm=POST, htu=/oauth2/token)'] },
        { from: 1, to: 0, label: ['200 OK — access token', '(token_type=DPoP, cnf.jkt = key thumbprint)'] },
        { from: 0, to: 2, label: 'GET /resource', sublabel: ['Authorization: DPoP <token>,', 'DPoP: <fresh proof JWT>'] },
        { from: 2, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function TokenIntrospectionDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client', 'Resource Server', productName]}
      gaps={[340, 340]}
      ariaLabel={`Token Introspection flow: a client presents a token to a resource server, which calls the introspection endpoint at ${productName} to check token validity and metadata before serving the request.`}
      rows={[
        { from: 0, to: 1, label: 'GET /resource', sublabel: 'Authorization: Bearer <token>' },
        { from: 1, to: 2, label: 'POST /oauth2/introspect', sublabel: 'token, client_auth' },
        { from: 2, to: 1, label: ['200 OK — { active, sub, scope,', 'aud, exp, client_id }'] },
        { from: 1, to: 0, label: '200 OK' },
      ]}
    />
  );
}

export function DCRDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Client', productName]}
      gaps={[560]}
      ariaLabel={`Dynamic Client Registration flow: a developer or automated tooling sends client metadata to the registration endpoint and ${productName} returns an assigned client_id and client_secret.`}
      rows={[
        { from: 0, to: 1, label: 'POST /oauth2/dcr/register', sublabel: 'redirect_uris, grant_types, client_name' },
        { from: 1, to: 0, label: ['201 Created', 'client_id, client_secret, ...'] },
      ]}
    />
  );
}

export function OIDCFlowDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['User', 'User Agent', 'Application', productName]}
      gaps={[280, 280, 280]}
      ariaLabel={`OpenID Connect Authorization Code flow: the user starts a sign-in, the application requests the openid scope, ${productName} issues an ID Token alongside the access token, and the application optionally calls the UserInfo endpoint for additional claims.`}
      rows={[
        { from: 0, to: 2, label: 'Initiate sign-in' },
        { from: 2, to: 1, label: ['302 Redirect to /oauth2/authorize', '(scope includes openid)'] },
        { from: 1, to: 3, label: 'GET /oauth2/authorize', sublabel: ['scope=openid profile email,', 'nonce=...'] },
        { from: 3, to: 1, label: 'Sign-in page' },
        { from: 0, to: 1, label: 'Submit credentials' },
        { from: 1, to: 3, label: 'POST credentials' },
        { from: 3, to: 1, label: '302 Redirect ?code=...' },
        { from: 1, to: 2, label: 'Callback with code' },
        { from: 2, to: 3, label: 'POST /oauth2/token' },
        { from: 3, to: 2, label: ['200 OK — access token + ID token', '(+ refresh token)'] },
        { from: 2, to: 3, label: 'GET /oauth2/userinfo', sublabel: ['Authorization:', 'Bearer <access_token>'] },
        { from: 3, to: 2, label: '200 OK — user claims' },
      ]}
    />
  );
}

export function RPInitiatedLogoutDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['User Agent', 'Application', productName]}
      gaps={[360, 360]}
      ariaLabel={`RP-Initiated Logout flow: the user signs out of the application, the application clears its local session and redirects the browser to the end_session_endpoint, ${productName} validates the request and runs the sign-out flow to terminate the SSO session, then redirects the browser to the post-logout redirect URI.`}
      rows={[
        { from: 0, to: 1, label: 'Sign out' },
        { note: 'Application clears its local session', between: [1, 1] },
        { from: 1, to: 0, label: ['302 Redirect to', '/oauth2/logout'] },
        { from: 0, to: 2, label: 'GET /oauth2/logout', sublabel: ['id_token_hint,', 'post_logout_redirect_uri, state'] },
        { note: 'Validate request, run the sign-out flow', between: [2, 2] },
        { note: 'Terminate SSO session, clear session cookie', between: [2, 2] },
        { from: 2, to: 0, label: ['302 Redirect to', 'post_logout_redirect_uri?state=...'] },
        { from: 0, to: 1, label: 'Land on the post-logout page' },
      ]}
    />
  );
}

export function UserInfoDiagram() {
  const productName = useProductName();

  return (
    <SequenceDiagram
      actors={['Application', productName]}
      gaps={[560]}
      ariaLabel="UserInfo flow: the application presents an access token to the UserInfo endpoint and receives the authenticated user's claims, filtered by the granted scopes."
      rows={[
        { from: 0, to: 1, label: 'GET /oauth2/userinfo', sublabel: 'Authorization: Bearer <access_token>' },
        { from: 1, to: 0, label: ['200 OK — claims', '(JSON | JWS | JWE | NESTED_JWT)'] },
      ]}
    />
  );
}
