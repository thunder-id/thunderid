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

import { useState, useEffect, useRef, useCallback } from "react";

const REFRESH_MS = 3000;

function fmtTime(iso) {
    if (!iso) return "";
    try {
        const d = new Date(iso);
        const sameDay = d.toDateString() === new Date().toDateString();
        return sameDay
            ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
            : d.toLocaleDateString([], { month: "short", day: "numeric" });
    } catch {
        return "";
    }
}

function EnvelopeIcon({ size = 18 }) {
    return (
        <svg
            width={size}
            height={size}
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            viewBox="0 0 24 24"
            aria-hidden="true"
        >
            <rect x="2" y="4" width="20" height="16" rx="2" />
            <path d="m2 8 10 6 10-6" />
        </svg>
    );
}

function EmptyState() {
    return (
        <div className="empty-state">
            <EnvelopeIcon size={48} />
            <p>No emails yet.</p>
        </div>
    );
}

function MessageItem({ message, selected, onClick }) {
    const cls = [
        "message-item",
        message.read ? "" : "unread",
        selected ? "selected" : "",
    ]
        .filter(Boolean)
        .join(" ");

    return (
        <button type="button" className={cls} onClick={onClick}>
            <div className="msg-row1">
                <span className="msg-from">
                    {!message.read && (
                        <span className="unread-dot" aria-hidden="true" />
                    )}
                    {message.from || "(no sender)"}
                </span>
                <span className="msg-time">{fmtTime(message.receivedAt)}</span>
            </div>
            <div className="msg-subject">
                {message.subject || "(no subject)"}
            </div>
        </button>
    );
}

function MessageDetail({
    message,
    activeTab,
    onTabChange,
    onMarkUnread,
    onDelete,
}) {
    const iframeRef = useRef(null);

    useEffect(() => {
        if (activeTab === "body" && message?.html && iframeRef.current) {
            const doc = iframeRef.current.contentDocument;
            doc.open();
            // Force every link and form in the email to open in a new tab.
            doc.write('<base target="_blank" />' + message.html);
            doc.close();
        }
    }, [message, activeTab]);

    const tabs = [
        { id: "body", label: "Message" },
        { id: "headers", label: "Headers" },
        { id: "raw", label: "Raw" },
    ];

    return (
        <div className="detail-content">
            <div className="detail-header">
                <div className="detail-subject">
                    {message.subject || "(no subject)"}
                </div>
                <div className="detail-meta">
                    <div>
                        <strong>From:</strong> {message.from || "(unknown)"}
                    </div>
                    <div>
                        <strong>To:</strong> {(message.to || []).join(", ")}
                    </div>
                    <div>
                        <strong>Date:</strong>{" "}
                        {message.date || message.receivedAt || ""}
                    </div>
                </div>
                <div className="detail-actions">
                    <button className="btn btn-outline" onClick={onMarkUnread}>
                        Mark as unread
                    </button>
                    <button className="btn btn-danger" onClick={onDelete}>
                        Delete
                    </button>
                </div>
            </div>

            <div className="tabs">
                {tabs.map((tab) => (
                    <button
                        key={tab.id}
                        className={`tab-btn${activeTab === tab.id ? " active" : ""}`}
                        onClick={() => onTabChange(tab.id)}
                    >
                        {tab.label}
                    </button>
                ))}
            </div>

            {activeTab === "body" && (
                <div className="body-frame-wrap">
                    {message.html ? (
                        <iframe
                            ref={iframeRef}
                            sandbox="allow-same-origin allow-popups allow-popups-to-escape-sandbox"
                            title="Email body"
                        />
                    ) : (
                        <div className="body-text">
                            {message.text || "(empty)"}
                        </div>
                    )}
                </div>
            )}

            {activeTab === "headers" && (
                <div className="headers-panel">
                    {Object.entries(message.headers || {}).map(([k, v]) => (
                        <div key={k} className="header-row">
                            <span className="header-key">{k}</span>
                            <span className="header-val">
                                {Array.isArray(v) ? v.join("; ") : v}
                            </span>
                        </div>
                    ))}
                </div>
            )}

            {activeTab === "raw" && (
                <div className="raw-panel">{message.raw || ""}</div>
            )}
        </div>
    );
}

export default function App() {
    const [messages, setMessages] = useState([]);
    const [unread, setUnread] = useState(0);
    const [selectedId, setSelectedId] = useState(null);
    const [selectedMsg, setSelectedMsg] = useState(null);
    const [activeTab, setActiveTab] = useState("body");

    const fetchList = useCallback(async () => {
        try {
            const res = await fetch("/api/messages");
            const data = await res.json();
            setMessages(data.messages ?? []);
            setUnread(data.unread ?? 0);
        } catch {}
    }, []);

    useEffect(() => {
        fetchList();
        const id = setInterval(fetchList, REFRESH_MS);
        return () => clearInterval(id);
    }, [fetchList]);

    async function openMessage(id) {
        const wasUnread = messages.find((m) => m.id === id)?.read === false;
        setSelectedId(id);
        setActiveTab("body");
        try {
            const res = await fetch(`/api/messages/${id}`);
            if (!res.ok) {
                setSelectedId(null);
                setSelectedMsg(null);
                return;
            }
            const { message } = await res.json();
            setSelectedMsg(message);
            setMessages((prev) =>
                prev.map((m) => (m.id === id ? { ...m, read: true } : m)),
            );
            if (wasUnread) setUnread((prev) => Math.max(0, prev - 1));
        } catch {
            setSelectedId(null);
            setSelectedMsg(null);
        }
    }

    async function handleMarkUnread(id) {
        await fetch(`/api/messages/${id}/unread`, { method: "POST" });
        const wasRead = messages.find((m) => m.id === id)?.read === true;
        setMessages((prev) =>
            prev.map((m) => (m.id === id ? { ...m, read: false } : m)),
        );
        if (wasRead) setUnread((prev) => prev + 1);
    }

    async function handleDelete(id) {
        await fetch(`/api/messages/${id}`, { method: "DELETE" });
        setSelectedId(null);
        setSelectedMsg(null);
        await fetchList();
    }

    async function handleClearAll() {
        if (!confirm("Delete all messages?")) return;
        await fetch("/api/messages", { method: "DELETE" });
        setSelectedId(null);
        setSelectedMsg(null);
        await fetchList();
    }

    return (
        <>
            <header className="site-header">
                <div className="brand">
                    <div className="brand-mark">
                        <EnvelopeIcon />
                    </div>
                    <span className="brand-name">
                        Wayfinder <span>Mail</span>
                    </span>
                    {unread > 0 && (
                        <span
                            className="unread-badge"
                            aria-label={`${unread} unread`}
                        >
                            {unread}
                        </span>
                    )}
                </div>
                <div className="header-right">
                    <button className="btn btn-danger" onClick={handleClearAll}>
                        Clear all
                    </button>
                </div>
            </header>

            <div className="layout">
                <div className="sidebar">
                    <div className="sidebar-heading">Inbox</div>
                    <div className="message-list">
                        {messages.length === 0 ? (
                            <EmptyState />
                        ) : (
                            messages.map((m) => (
                                <MessageItem
                                    key={m.id}
                                    message={m}
                                    selected={m.id === selectedId}
                                    onClick={() => openMessage(m.id)}
                                />
                            ))
                        )}
                    </div>
                </div>

                <div className="detail">
                    {selectedMsg ? (
                        <MessageDetail
                            message={selectedMsg}
                            activeTab={activeTab}
                            onTabChange={setActiveTab}
                            onMarkUnread={() =>
                                handleMarkUnread(selectedMsg.id)
                            }
                            onDelete={() => handleDelete(selectedMsg.id)}
                        />
                    ) : (
                        <div className="detail-placeholder">
                            <EnvelopeIcon size={48} />
                            <span>Select an email to read it</span>
                        </div>
                    )}
                </div>
            </div>
        </>
    );
}
