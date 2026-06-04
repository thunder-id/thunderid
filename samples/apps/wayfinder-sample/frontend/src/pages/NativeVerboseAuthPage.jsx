/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { AcceptInvite, Recovery, SignIn, SignUp, resolveFlowTemplateLiterals, useThunderID, useTranslation } from "@thunderid/react";
import { useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useNativeAuth } from "../auth/NativeAuthContext";
import { exchangeAssertion } from "../auth/nativeAuthService";

const AUTH_SERVER_BASE_URL = import.meta.env.VITE_THUNDER_BASE_URL || "";

function extractManagedLinkCopy(html, fallbackPrefix, fallbackLabel) {
  if (typeof html !== "string") {
    return { linkLabel: fallbackLabel, prefix: fallbackPrefix };
  }

  const match = html.match(/<span[^>]*>(.*?)<\/span>\s*<a[^>]*>.*?<span[^>]*>(.*?)<\/span>/s);

  return {
    linkLabel: match?.[2] || fallbackLabel,
    prefix: match?.[1] || fallbackPrefix,
  };
}

function NativeVerboseInlineLink({ labelHtml, meta, path, fallbackLabel, fallbackPrefix }) {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const resolvedLabelHtml = resolveFlowTemplateLiterals(labelHtml, { meta, t });
  const { linkLabel, prefix } = extractManagedLinkCopy(resolvedLabelHtml, fallbackPrefix, fallbackLabel);

  return (
    <p style={{ margin: 0 }}>
      <span>{prefix}</span>
      <a
        href={path}
        className="rich-text-link"
        onClick={(event) => {
          event.preventDefault();
          navigate(path);
        }}
      >
        {linkLabel}
      </a>
    </p>
  );
}

export const nativeVerboseAuthExtensions = {
  components: {
    renderers: {
      rich_text_forgot_password: (component, context) => (
        <NativeVerboseInlineLink
          labelHtml={component.label}
          meta={context.meta}
          path="/recovery"
          fallbackLabel="Reset password"
          fallbackPrefix="Forgot your password? "
        />
      ),
      rich_text_signup: (component, context) => (
        <NativeVerboseInlineLink
          labelHtml={component.label}
          meta={context.meta}
          path={context.authType === "recovery" ? "/signin" : "/signup"}
          fallbackLabel={context.authType === "recovery" ? "Sign in" : "Sign up"}
          fallbackPrefix={
            context.authType === "recovery"
              ? "Remember your password? "
              : "Don't have an account? "
          }
        />
      ),
    },
  },
};

export function NativeVerboseAuthPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { setToken } = useNativeAuth();
  const { getAccessToken: getSDKToken } = useThunderID();
  const [globalError, setGlobalError] = useState(null);
  const [isCompletingSignIn, setIsCompletingSignIn] = useState(false);

  const pathToMode = { "/signin": "signin", "/signup": "signup", "/recovery": "recovery" };
  const mode = pathToMode[location.pathname] || "signin";

  const urlParams = new URLSearchParams(location.search);
  const executionId = urlParams.get("executionId");
  const inviteToken = urlParams.get("inviteToken");
  const isInviteFlow = mode === "signup" && Boolean(executionId && inviteToken);

  async function handleSignUpComplete() {
    setIsCompletingSignIn(true);
    setGlobalError(null);
    try {
      const assertion = await getSDKToken();
      if (!assertion) {
        throw new Error("Sign-up completed but no credentials were returned.");
      }
      const tokenResult = await exchangeAssertion(assertion);
      setToken(tokenResult.access_token);
      navigate("/flights", { replace: true });
    } catch (error) {
      setGlobalError(error.message || "Failed to complete sign-in after registration. Please sign in manually.");
      setIsCompletingSignIn(false);
    }
  }

  async function handleSignInSuccess() {
    setIsCompletingSignIn(true);
    setGlobalError(null);
    try {
      // The SDK stores the flow assertion as its session token before calling onSuccess.
      // Exchange it for a proper OAuth2 access token for resource API calls.
      const assertion = await getSDKToken();
      if (!assertion) {
        throw new Error("Sign-in completed but no credentials were returned.");
      }
      const tokenResult = await exchangeAssertion(assertion);
      setToken(tokenResult.access_token);
      navigate("/flights", { replace: true });
    } catch (error) {
      setGlobalError(error.message || "Failed to complete sign-in. Please try again.");
      setIsCompletingSignIn(false);
    }
  }

  function renderContent() {
    if (globalError) {
      return (
        <>
          <div role="alert" className="native-auth-error">
            {globalError}
          </div>
          <button
            type="button"
            className="native-auth-btn-primary"
            onClick={() => { setGlobalError(null); navigate("/signin"); }}
          >
            Try again
          </button>
        </>
      );
    }

    if (isCompletingSignIn) {
      return <p style={{ margin: 0, color: "#475569" }}>Completing sign in…</p>;
    }

    if (isInviteFlow) {
      return (
        <AcceptInvite
          baseUrl={AUTH_SERVER_BASE_URL}
          executionId={executionId}
          inviteToken={inviteToken}
          onGoToSignIn={() => navigate("/signin")}
        />
      );
    }

    if (mode === "signup") {
      return <SignUp onComplete={handleSignUpComplete} afterSignUpUrl={`${window.location.origin}/signin`} />;
    }

    if (mode === "recovery") {
      return (
        <Recovery
          afterRecoveryUrl={`${window.location.origin}/signin`}
          tokenUrlParam="inviteToken"
        />
      );
    }

    return <SignIn  onSuccess={handleSignInSuccess} />;
  }

  return <main className="native-auth-page">{renderContent()}</main>;
}
