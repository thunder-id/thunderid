/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { useNativeAuth } from "../auth/NativeAuthContext";
import {
  exchangeAssertion,
  getAppMetadata,
  initiateFlow,
  submitFlowStep,
} from "../auth/nativeAuthService";

const FLOW_TYPE_MAP = {
  signin: "AUTHENTICATION",
  signup: "REGISTRATION",
  recovery: "RECOVERY",
};

const MODE_TITLE = {
  signin: "Sign in",
  signup: "Create account",
  recovery: "Reset password",
};

const SK_EXECUTION_ID = "wf_execution_id";
const SK_CHALLENGE_TOKEN = "wf_challenge_token";
const SK_SOCIAL_PENDING = "wf_social_pending";

export function NativeAuthPage() {
  const location = useLocation();
  const navigate = useNavigate();
  const { setToken } = useNativeAuth();

  const pathToMode = { "/signin": "signin", "/signup": "signup", "/recovery": "recovery" };
  const mode = pathToMode[location.pathname] || "signin";

  const [flowState, setFlowState] = useState(null);
  const [formValues, setFormValues] = useState({});
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState(null);
  const [appMeta, setAppMeta] = useState(null);
  const [registrationDone, setRegistrationDone] = useState(false);

  const initialized = useRef(false);

  useEffect(() => {
    getAppMetadata().then(setAppMeta).catch(() => {});
  }, []);

  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;

    const urlParams = new URLSearchParams(window.location.search);
    const code = urlParams.get("code");
    const isPendingSocial = sessionStorage.getItem(SK_SOCIAL_PENDING) === "true";
    const executionId = urlParams.get("executionId") || urlParams.get("id");

    if (code && isPendingSocial) {
      handleSocialCallback(code);
    } else if (executionId) {
      resumeFlowFromLink(executionId, extractUrlHiddenValues(urlParams));
    } else {
      startFlow(mode);
    }
  }, []);

  function extractUrlHiddenValues(urlParams) {
    const skip = new Set(["code", "state", "executionId", "id", "applicationId"]);
    const hidden = {};
    for (const [key, value] of urlParams.entries()) {
      if (!skip.has(key)) hidden[key] = value;
    }
    return hidden;
  }

  async function resumeFlowFromLink(executionId, hiddenValues = {}) {
    window.history.replaceState({}, "", window.location.pathname);
    setIsLoading(true);
    setError(null);
    try {
      const inputs = Object.keys(hiddenValues).length > 0 ? hiddenValues : undefined;
      await processResponse(await submitFlowStep({ executionId, inputs }), hiddenValues);
    } catch (err) {
      setError(err.message || "Invalid or expired link. Please try again.");
    } finally {
      setIsLoading(false);
    }
  }

  async function startFlow(flowMode) {
    setIsLoading(true);
    setError(null);
    setFlowState(null);
    setFormValues({});
    try {
      await processResponse(await initiateFlow(FLOW_TYPE_MAP[flowMode] || "AUTHENTICATION"));
    } catch (err) {
      setError(err.message || "Failed to start authentication. Please try again.");
    } finally {
      setIsLoading(false);
    }
  }

  async function processResponse(response, pendingHiddenValues = {}) {
    const { flowStatus, type, data = {}, assertion, executionId, challengeToken, failureReason } =
      response;

    if (flowStatus === "COMPLETE") {
      if (!assertion) {
        if (mode === "signup") {
          setFlowState(null);
          setRegistrationDone(true);
        } else {
          navigate("/signin");
        }
        return;
      }
      setIsLoading(true);
      try {
        const tokenResult = await exchangeAssertion(assertion);
        setToken(tokenResult.access_token);
        navigate("/flights", { replace: true });
      } catch (err) {
        setError(err.message || "Failed to complete sign-in. Please try again.");
        setIsLoading(false);
      }
      return;
    }

    if (flowStatus === "ERROR") {
      setError(failureReason || "An authentication error occurred. Please try again.");
      setFlowState(null);
      return;
    }

    setError(failureReason || null);

    if (type === "REDIRECTION") {
      if (!data.redirectURL) { setError("Social login redirect failed."); return; }
      sessionStorage.setItem(SK_EXECUTION_ID, executionId || "");
      sessionStorage.setItem(SK_CHALLENGE_TOKEN, challengeToken || "");
      sessionStorage.setItem(SK_SOCIAL_PENDING, "true");
      window.location.href = data.redirectURL;
      return;
    }

    const inputs = data.inputs || [];
    const actions = data.actions || [];
    const hiddenInputs = inputs.filter((i) => i.type === "HIDDEN");
    const seedValues = {};

    hiddenInputs.forEach((input) => {
      if (pendingHiddenValues[input.identifier] !== undefined) {
        seedValues[input.identifier] = pendingHiddenValues[input.identifier];
      }
    });
    if (Object.keys(seedValues).length > 0) {
      setFormValues((prev) => ({ ...prev, ...seedValues }));
    }

    const visibleInputs = inputs.filter((i) => i.type !== "HIDDEN");
    if (actions.length === 1 && visibleInputs.length === 0) {
      await submitStep(actions[0].ref, seedValues, executionId, challengeToken);
      return;
    }

    setFlowState({ executionId, challengeToken, inputs, actions });
  }

  async function handleSocialCallback(code) {
    sessionStorage.removeItem(SK_SOCIAL_PENDING);
    const executionId = sessionStorage.getItem(SK_EXECUTION_ID);
    const challengeToken = sessionStorage.getItem(SK_CHALLENGE_TOKEN);
    const url = new URL(window.location.href);
    url.searchParams.delete("code");
    url.searchParams.delete("state");
    window.history.replaceState({}, "", url.pathname + (url.search || ""));
    setIsLoading(true);
    setError(null);
    try {
      await processResponse(
        await submitFlowStep({
          executionId,
          inputs: { type: "SOCIAL", code },
          challengeToken: challengeToken || undefined,
        })
      );
    } catch (err) {
      setError(err.message || "Social login failed. Please try again.");
    } finally {
      setIsLoading(false);
    }
  }

  async function submitStep(actionRef, extraInputs, executionId, challengeToken) {
    setIsLoading(true);
    setError(null);
    try {
      await processResponse(
        await submitFlowStep({
          executionId: executionId ?? flowState?.executionId,
          action: actionRef,
          inputs: extraInputs && Object.keys(extraInputs).length > 0 ? extraInputs : undefined,
          challengeToken: challengeToken ?? flowState?.challengeToken,
        })
      );
    } catch (err) {
      setError(err.message || "Something went wrong. Please try again.");
    } finally {
      setIsLoading(false);
    }
  }

  function handleFieldChange(identifier, value) {
    setFormValues((prev) => ({ ...prev, [identifier]: value }));
  }

  async function handleFormSubmit(event, actionRef) {
    event.preventDefault();
    await submitFormWithAction(actionRef);
  }

  async function submitFormWithAction(actionRef) {
    const inputs = flowState?.inputs || [];
    const visibleInputs = inputs.filter((i) => i.type !== "HIDDEN");
    const hiddenInputs = inputs.filter((i) => i.type === "HIDDEN");

    for (const input of visibleInputs) {
      if (input.required && !formValues[input.identifier]) {
        setError(`${labelFor(input)} is required.`);
        return;
      }
    }
    setError(null);

    const allInputs = {};
    for (const input of [...visibleInputs, ...hiddenInputs]) {
      if (formValues[input.identifier] !== undefined) {
        allInputs[input.identifier] = formValues[input.identifier];
      }
    }

    await submitStep(actionRef, allInputs);
  }

  return (
    <main className="native-auth-page">
      <div className="native-auth-card">
        {appMeta && <AppBranding meta={appMeta} />}

        {registrationDone ? (
          <RegistrationSuccess onSignIn={() => navigate("/signin")} />
        ) : (
          <>
            <h1 className="native-auth-heading">{MODE_TITLE[mode] || "Sign in"}</h1>

            {error && (
              <div className="native-auth-error" role="alert">
                {error}
              </div>
            )}

            {isLoading && !flowState && <p className="native-auth-status">Loading…</p>}

            {flowState && (
              <FlowStep
                flowState={flowState}
                formValues={formValues}
                isLoading={isLoading}
                mode={mode}
                onFieldChange={handleFieldChange}
                onFormSubmit={handleFormSubmit}
                onFormActionClick={(ref) => submitFormWithAction(ref)}
                onActionClick={(ref) => submitStep(ref, {})}
              />
            )}

            <div className="native-auth-mode-links">
              {mode === "signin" && (
                <>
                  <span>
                    Don&apos;t have an account?{" "}
                    <button
                      type="button"
                      className="native-auth-link-btn"
                      onClick={() => navigate("/signup")}
                    >
                      Create one
                    </button>
                  </span>
                  <span>
                    <button
                      type="button"
                      className="native-auth-link-btn"
                      onClick={() => navigate("/recovery")}
                    >
                      Forgot your password?
                    </button>
                  </span>
                </>
              )}
              {mode === "signup" && (
                <span>
                  Already have an account?{" "}
                  <button
                    type="button"
                    className="native-auth-link-btn"
                    onClick={() => navigate("/signin")}
                  >
                    Sign in
                  </button>
                </span>
              )}
              {mode === "recovery" && (
                <span>
                  Remember your password?{" "}
                  <button
                    type="button"
                    className="native-auth-link-btn"
                    onClick={() => navigate("/signin")}
                  >
                    Sign in
                  </button>
                </span>
              )}
            </div>
          </>
        )}
      </div>
    </main>
  );
}

function RegistrationSuccess({ onSignIn }) {
  return (
    <div className="native-auth-success">
      <span className="native-auth-success-icon">✅</span>
      <h2 className="native-auth-success-heading">Account created!</h2>
      <p className="native-auth-success-message">Your account is ready. Sign in to continue.</p>
      <button type="button" className="native-auth-btn-primary" onClick={onSignIn}>
        Sign in
      </button>
    </div>
  );
}

function AppBranding({ meta }) {
  const logo = renderLogo(meta.logoUrl);
  return (
    <div className="native-auth-branding">
      {logo && <div className="native-auth-logo-wrap">{logo}</div>}
      {meta.name && <span className="native-auth-brand-name">{meta.name}</span>}
    </div>
  );
}

function renderLogo(logoUrl) {
  if (!logoUrl) return null;
  if (logoUrl.startsWith("emoji:")) {
    return (
      <span className="native-auth-emoji-logo" aria-hidden="true">
        {logoUrl.slice("emoji:".length)}
      </span>
    );
  }
  return <img src={logoUrl} alt="App logo" className="native-auth-img-logo" />;
}

function FlowStep({
  flowState,
  formValues,
  isLoading,
  mode,
  onFieldChange,
  onFormSubmit,
  onFormActionClick,
  onActionClick,
}) {
  const { inputs = [], actions = [] } = flowState;
  const visibleInputs = inputs.filter((i) => i.type !== "HIDDEN");

  if (actions.length > 1 && visibleInputs.length === 0) {
    return (
      <div className="native-auth-form-body">
        <div className="native-auth-action-group">
          {actions.map((action) => (
            <button
              key={action.ref}
              type="button"
              className={actionBtnClass(action)}
              disabled={isLoading}
              onClick={() => onActionClick(action.ref)}
            >
              {isLoading ? "Loading…" : labelForAction(action, mode)}
            </button>
          ))}
        </div>
      </div>
    );
  }

  if (actions.length > 1 && visibleInputs.length > 0) {
    return (
      <div className="native-auth-form-body">
        {visibleInputs.map((input) => (
          <FlowField
            key={input.identifier}
            input={input}
            value={formValues[input.identifier] || ""}
            onChange={(val) => onFieldChange(input.identifier, val)}
            disabled={isLoading}
          />
        ))}
        <div className="native-auth-action-group">
          {actions.map((action) => (
            <button
              key={action.ref}
              type="button"
              className={actionBtnClass(action)}
              disabled={isLoading}
              onClick={() => onFormActionClick(action.ref)}
            >
              {isLoading ? "Loading…" : labelForAction(action, mode)}
            </button>
          ))}
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={(e) => onFormSubmit(e, actions[0]?.ref)} className="native-auth-form-body">
      {visibleInputs.map((input) => (
        <FlowField
          key={input.identifier}
          input={input}
          value={formValues[input.identifier] || ""}
          onChange={(val) => onFieldChange(input.identifier, val)}
          disabled={isLoading}
        />
      ))}
      {actions.length === 1 && (
        <button type="submit" className="native-auth-btn-primary" disabled={isLoading}>
          {isLoading ? "Loading…" : submitLabel(actions[0]?.ref, visibleInputs, mode)}
        </button>
      )}
    </form>
  );
}

function FlowField({ input, value, onChange, disabled }) {
  const type = resolveInputType(input.type);
  return (
    <label className="native-auth-field-label">
      <span className="native-auth-field-label-text">
        {labelFor(input)}
        {input.required && <span className="native-auth-required"> *</span>}
      </span>
      <input
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        required={input.required}
        autoComplete={
          type === "password" ? "current-password" : type === "email" ? "email" : "off"
        }
        className="native-auth-input"
      />
    </label>
  );
}

function resolveInputType(type) {
  switch (type) {
    case "PASSWORD_INPUT": return "password";
    case "EMAIL_INPUT": return "email";
    case "OTP_INPUT": return "text";
    case "PHONE_INPUT": return "tel";
    default: return "text";
  }
}

function labelFor(input) {
  return input.identifier
    .replace(/_/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .replace(/^./, (c) => c.toUpperCase());
}

function labelForAction(action, mode = "signin") {
  const ref = action?.ref || "";
  const prefix = mode === "signup" ? "Sign up" : "Continue";
  if (ref.includes("google")) return `${prefix} with Google`;
  if (ref.includes("github")) return `${prefix} with GitHub`;
  if (ref.includes("facebook")) return `${prefix} with Facebook`;
  if (ref.includes("mobile") || ref.includes("sms")) return `${prefix} with SMS OTP`;
  if (ref === "consent_action_allow" || ref.endsWith("_allow")) return "Allow";
  if (ref === "consent_action_deny" || ref.endsWith("_deny")) return "Deny";
  const cleaned = ref.replace(/^action_role_/, "").replace(/^action_/, "").replace(/_/g, " ").trim();
  if (!cleaned || /^\d+$/.test(cleaned)) return "Continue";
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
}

function submitLabel(actionRef, visibleInputs, mode) {
  const ref = (actionRef || "").toLowerCase();
  if (ref.includes("signin") || ref.includes("sign_in")) return "Sign in";
  if (ref.includes("signup") || ref.includes("sign_up")) return "Create account";
  const hasPassword = visibleInputs.some(
    (i) => i.type === "PASSWORD_INPUT" || i.identifier === "password"
  );
  const hasOtp = visibleInputs.some((i) => i.type === "OTP_INPUT" || i.identifier === "otp");
  if (hasOtp) return "Verify OTP";
  if (hasPassword) {
    if (mode === "recovery") return "Reset password";
    if (mode === "signup") return "Create account";
    return "Sign in";
  }
  if (ref.includes("submit_username")) return "Send recovery link";
  if (ref.includes("submit_email")) return "Send invitation";
  if (ref.includes("user_details") || ref.includes("schema_attrs")) return "Finish";
  return "Continue";
}

function actionBtnClass(action) {
  const ref = action?.ref || "";
  if (ref.endsWith("_deny") || ref.endsWith("_reject") || ref === "consent_action_deny") {
    return "native-auth-btn-secondary";
  }
  return "native-auth-btn-primary";
}
