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

import { useCallback, useEffect, useRef, useState } from "react";
import { useThunderID } from "@thunderid/react";
import { KeyRound, RefreshCw, Save } from "lucide-react";
import { getMyUser, updateMyCredentials, updateMyUser } from "../api/userApi";

export function ProfilePage() {
  const { isSignedIn, isLoading: authLoading, signIn, getAccessToken } = useThunderID();

  if (authLoading) {
    return (
      <main className="bookings-page">
        <p className="empty-state management-message">Loading…</p>
      </main>
    );
  }

  if (!isSignedIn) {
    return (
      <main className="bookings-page">
        <section className="management-empty">
          <div>
            <p className="eyebrow">Profile</p>
            <h1>Sign in to manage your profile.</h1>
            <p>View and update your account attributes and password.</p>
          </div>
          <button
            className="dashboard-action dashboard-action--secondary"
            type="button"
            onClick={() => signIn({ acr_values: "urn:thunder:auth:user" })}
          >
            Sign in
          </button>
        </section>
      </main>
    );
  }

  return <Profile getAccessToken={getAccessToken} />;
}

function flattenAttributes(attrs, prefix = "") {
  const rows = [];
  if (attrs == null || typeof attrs !== "object" || Array.isArray(attrs)) {
    return rows;
  }
  for (const [key, value] of Object.entries(attrs)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (value != null && typeof value === "object" && !Array.isArray(value)) {
      rows.push(...flattenAttributes(value, path));
    } else {
      rows.push({
        path,
        value: value == null ? "" : Array.isArray(value) ? JSON.stringify(value) : String(value)
      });
    }
  }
  return rows;
}

function unflattenAttributes(rows) {
  const result = {};
  for (const { path, value } of rows) {
    const segments = path.split(".");
    let cursor = result;
    for (let i = 0; i < segments.length - 1; i += 1) {
      const segment = segments[i];
      if (cursor[segment] == null || typeof cursor[segment] !== "object") {
        cursor[segment] = {};
      }
      cursor = cursor[segment];
    }
    cursor[segments[segments.length - 1]] = value;
  }
  return result;
}

function Profile({ getAccessToken }) {
  const getAccessTokenRef = useRef(getAccessToken);
  useEffect(() => {
    getAccessTokenRef.current = getAccessToken;
  }, [getAccessToken]);

  const [user, setUser] = useState(null);
  const [rows, setRows] = useState([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  const loadProfile = useCallback(async () => {
    setIsLoading(true);
    setError("");
    setMessage("");
    try {
      const accessToken = await getAccessTokenRef.current();
      const data = await getMyUser(accessToken);
      setUser(data);
      setRows(flattenAttributes(data?.attributes || {}));
    } catch (err) {
      setError(formatError(err));
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadProfile();
  }, [loadProfile]);

  function updateRow(index, nextValue) {
    setRows((current) =>
      current.map((row, i) => (i === index ? { ...row, value: nextValue } : row))
    );
  }

  async function handleSaveAttributes(event) {
    event.preventDefault();
    setError("");
    setMessage("");
    setIsSaving(true);
    try {
      const accessToken = await getAccessTokenRef.current();
      const updated = await updateMyUser(accessToken, unflattenAttributes(rows));
      setUser(updated);
      setRows(flattenAttributes(updated?.attributes || {}));
      setMessage("Profile updated.");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <main className="bookings-page">
      <section className="management-header">
        <div>
          <p className="eyebrow">Account</p>
          <h1>Profile</h1>
          <p style={{ marginTop: 4, color: "#666" }}>
            View your account details and update your profile attributes.
          </p>
        </div>
        <button
          className="dashboard-action dashboard-action--secondary"
          type="button"
          onClick={loadProfile}
          disabled={isLoading || isSaving}
        >
          <RefreshCw size={16} /> Refresh
        </button>
      </section>

      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}
      {message && (
        <div className="api-status" role="status">
          {message}
        </div>
      )}

      {isLoading && <p className="empty-state management-message">Loading profile…</p>}

      {!isLoading && user && (
        <>
          <section className="management-panel" aria-label="Account details" style={profilePanelStyle}>
            <h2 style={sectionHeadingStyle}>Account details</h2>
            <dl style={metaListStyle}>
              <ReadOnlyRow label="User ID" value={user.id} />
              <ReadOnlyRow label="Type" value={user.type || "—"} />
              <ReadOnlyRow label="Organization Unit" value={user.ouHandle || user.ouId || "—"} />
            </dl>
          </section>

          <section className="management-panel" aria-label="Profile attributes" style={profilePanelStyle}>
            <h2 style={sectionHeadingStyle}>Profile attributes</h2>
            {rows.length === 0 ? (
              <p style={{ color: "#666" }}>No attributes are set on this user yet.</p>
            ) : (
              <form onSubmit={handleSaveAttributes}>
                <div style={rowGridStyle}>
                  {rows.map((row, index) => (
                    <label key={row.path} style={fieldStyle}>
                      <span style={fieldLabelStyle}>{row.path}</span>
                      <input
                        type="text"
                        value={row.value}
                        onChange={(event) => updateRow(index, event.target.value)}
                        disabled={isSaving || user.isReadOnly}
                        style={inputStyle}
                      />
                    </label>
                  ))}
                </div>
                <div style={{ marginTop: 16, display: "flex", gap: 8 }}>
                  <button
                    className="dashboard-action"
                    type="submit"
                    disabled={isSaving || user.isReadOnly}
                  >
                    <Save size={16} /> {isSaving ? "Saving…" : "Save changes"}
                  </button>
                </div>
                {user.isReadOnly && (
                  <p style={{ color: "#666", marginTop: 8 }}>
                    This user is read-only and cannot be edited.
                  </p>
                )}
              </form>
            )}
          </section>

          <PasswordSection getAccessTokenRef={getAccessTokenRef} disabled={user.isReadOnly} />
        </>
      )}
    </main>
  );
}

function PasswordSection({ getAccessTokenRef, disabled }) {
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");

  async function handleSubmit(event) {
    event.preventDefault();
    setError("");
    setMessage("");

    if (password.length < 8) {
      setError("Password must be at least 8 characters.");
      return;
    }
    if (password !== confirm) {
      setError("Passwords do not match.");
      return;
    }

    setIsSaving(true);
    try {
      const accessToken = await getAccessTokenRef.current();
      await updateMyCredentials(accessToken, { password });
      setPassword("");
      setConfirm("");
      setMessage("Password updated.");
    } catch (err) {
      setError(formatError(err));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <section className="management-panel" aria-label="Change password" style={profilePanelStyle}>
      <h2 style={sectionHeadingStyle}>
        <KeyRound size={18} style={{ verticalAlign: "-3px", marginRight: 6 }} />
        Change password
      </h2>
      {error && (
        <div className="api-status api-status--error" role="status">
          {error}
        </div>
      )}
      {message && (
        <div className="api-status" role="status">
          {message}
        </div>
      )}
      <form onSubmit={handleSubmit}>
        <div style={rowGridStyle}>
          <label style={fieldStyle}>
            <span style={fieldLabelStyle}>New password</span>
            <input
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              disabled={disabled || isSaving}
              style={inputStyle}
            />
          </label>
          <label style={fieldStyle}>
            <span style={fieldLabelStyle}>Confirm new password</span>
            <input
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={(event) => setConfirm(event.target.value)}
              disabled={disabled || isSaving}
              style={inputStyle}
            />
          </label>
        </div>
        <div style={{ marginTop: 16 }}>
          <button
            className="dashboard-action"
            type="submit"
            disabled={disabled || isSaving || !password || !confirm}
          >
            <KeyRound size={16} /> {isSaving ? "Saving…" : "Update password"}
          </button>
        </div>
      </form>
    </section>
  );
}

function ReadOnlyRow({ label, value }) {
  return (
    <div style={metaRowStyle}>
      <dt style={metaLabelStyle}>{label}</dt>
      <dd style={metaValueStyle}>{value}</dd>
    </div>
  );
}

function formatError(err) {
  if (!err) return "Unknown error";
  if (err.status === 401) {
    return "Your session has expired. Please sign in again.";
  }
  return err.message || String(err);
}

const profilePanelStyle = {
  padding: 24,
  marginTop: 16,
  minHeight: 0
};

const sectionHeadingStyle = {
  marginTop: 0,
  marginBottom: 16,
  fontSize: "1.15rem"
};

const rowGridStyle = {
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))",
  gap: 12
};

const fieldStyle = {
  display: "flex",
  flexDirection: "column",
  gap: 6
};

const fieldLabelStyle = {
  fontSize: "0.78rem",
  fontWeight: 600,
  color: "#475569",
  textTransform: "uppercase",
  letterSpacing: "0.04em"
};

const inputStyle = {
  padding: "10px 12px",
  border: "1px solid #cbd5e1",
  borderRadius: 6,
  fontSize: "0.95rem",
  background: "#fff"
};

const metaListStyle = {
  margin: 0,
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))",
  gap: 12
};

const metaRowStyle = {
  display: "flex",
  flexDirection: "column",
  gap: 4
};

const metaLabelStyle = {
  fontSize: "0.78rem",
  fontWeight: 600,
  color: "#475569",
  textTransform: "uppercase",
  letterSpacing: "0.04em",
  margin: 0
};

const metaValueStyle = {
  margin: 0,
  fontFamily: "ui-monospace, Menlo, monospace",
  fontSize: "0.9rem",
  color: "#0f172a",
  wordBreak: "break-all"
};
