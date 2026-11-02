const panels = document.querySelectorAll("[data-view-panel]");
const registrationForm = document.querySelector("#registration-form");
const registrationResult = document.querySelector("#registration-result");
const loginForm = document.querySelector("#login-form");
const loginResult = document.querySelector("#login-result");
const postForm = document.querySelector("#post-form");
const postResult = document.querySelector("#post-result");
const postCategories = document.querySelector("#post-categories");
const postSubmitButton = postForm.querySelector('button[type="submit"]');
const feedStatus = document.querySelector("#feed-status");
const feedPosts = document.querySelector("#feed-posts");
const detailStatus = document.querySelector("#detail-status");
const detailPost = document.querySelector("#post-detail");
const commentsStatus = document.querySelector("#comments-status");
const postComments = document.querySelector("#post-comments");
const commentForm = document.querySelector("#comment-form");
const commentResult = document.querySelector("#comment-result");
const logoutButton = document.querySelector("[data-logout]");
const authStatus = document.querySelector("#auth-status");
const realtimeStatus = document.querySelector("#realtime-status");
const realtimeRetry = document.querySelector("#realtime-retry");
const contactStatus = document.querySelector("#contact-status");
const contactList = document.querySelector("#contact-list");
const chatWindowsElement = document.querySelector("#chat-windows");
const chatWindowTemplate = document.querySelector("#chat-window-template");
const chatToggles = document.querySelectorAll("[data-chat-toggle]");
const chatLauncher = document.querySelector("#chat-launcher");
const profileTitle = document.querySelector("#profile-title");
const profileStatus = document.querySelector("#profile-status");
const profileDetails = document.querySelector("#profile-details");
const profileForm = document.querySelector("#profile-form");
const guestNavigation = document.querySelectorAll("[data-guest-only]");
const authenticatedNavigation = document.querySelectorAll("[data-auth-required]");

let authenticated = false;
let activeView = "login";
let authGeneration = 0;
let categoriesLoaded = false;
let categoriesLoading = false;
let feedRequest = null;
let selectedPostID = null;
let realtimeSocket = null;
let realtimeRetryTimer = null;
let realtimeStableTimer = null;
let realtimeRetryCount = 0;
const onlineUsers = new Map();
let contacts = [];
let contactsRequest = null;
let contactsRefreshPending = false;
let presenceSnapshotReceived = false;
let currentUserID = 0;
const chatWindows = new Map();
let chatWindowSequence = 0;
const unreadMessages = new Map();
const seenMessageIDs = new Set();
let latestMessageNotice = "";
const maxRealtimeRetries = 5;

function showView(viewName) {
  activeView = viewName;
  for (const panel of panels) {
    panel.hidden = panel.dataset.viewPanel !== viewName;
  }
}

function setAuthenticated(value) {
  if (authenticated !== value) {
    authGeneration += 1;
  }
  authenticated = value;
  logoutButton.hidden = !authenticated;

  for (const button of guestNavigation) {
    button.hidden = authenticated;
  }

  for (const button of authenticatedNavigation) {
    button.hidden = !authenticated;
  }

  if (authenticated) {
    void loadCategories();
    clearOnlineUsers("Loading contacts…");
    void loadContacts();
    connectRealtimeSocket();
  } else {
    if (realtimeSocket || realtimeRetryTimer !== null) {
      closeRealtimeSocket();
    } else {
      setRealtimeStatus("");
      realtimeRetryCount = 0;
    }
    contacts = [];
    currentUserID = 0;
    unreadMessages.clear();
    seenMessageIDs.clear();
    latestMessageNotice = "";
    clearOnlineUsers("Sign in to see your contacts.");
    contactsRequest = null;
    contactsRefreshPending = false;
    categoriesLoaded = false;
    postForm.reset();
    postCategories.textContent = "Sign in to load categories.";
    postSubmitButton.disabled = true;
    feedPosts.replaceChildren();
    feedStatus.textContent = "";
    feedStatus.hidden = true;
    selectedPostID = null;
    detailPost.replaceChildren();
    postComments.replaceChildren();
    commentForm.hidden = true;
    for (const windowState of chatWindows.values()) {
      windowState.element.remove();
      windowState.minimizedButton?.remove();
    }
    chatWindows.clear();
    chatLauncher.hidden = true;
    profileForm.hidden = true;
    profileDetails.replaceChildren();
    profileStatus.textContent = "";
    setStatus(detailStatus, "");
    setStatus(commentsStatus, "");
  }
}

function setRealtimeStatus(message, showRetry = false) {
  realtimeStatus.textContent = message;
  realtimeStatus.hidden = !message;
  realtimeRetry.hidden = !showRetry;
}

function clearOnlineUsers(message) {
  onlineUsers.clear();
  presenceSnapshotReceived = false;
  for (const contact of contacts) contact.online = false;
  renderContacts(message);
}

function renderContacts(statusMessage = "") {
  contactList.replaceChildren();
  let onlineCount = 0;
  for (const contact of contacts) {
    if (contact.online) onlineCount += 1;
    const item = document.createElement("li");
    item.className = "contact-row";
    const dot = document.createElement("span");
    dot.className = `presence-dot ${contact.online ? "online" : "offline"}`;
    dot.setAttribute("aria-hidden", "true");
    const name = document.createElement("button");
    name.className = "contact-select";
    name.type = "button";
    name.dataset.openContact = "true";
    name.dataset.contactId = String(contact.id);
    name.dataset.contactUsername = contact.username;
    name.dataset.conversationId = String(contact.conversation_id);
    name.textContent = contact.username;
    const profileButton = document.createElement("button");
    profileButton.className = "contact-profile";
    profileButton.type = "button";
    profileButton.dataset.openProfile = String(contact.id);
    profileButton.textContent = "Profile";
    const status = document.createElement("span");
    status.className = "contact-presence";
    status.textContent = contact.online ? "Online" : "Offline";
    item.append(dot, name, profileButton);
    const unread = unreadMessages.get(String(contact.id)) || 0;
    if (unread > 0) {
      const badge = document.createElement("span");
      badge.className = "contact-unread";
      badge.textContent = `${unread} new`;
      badge.setAttribute("aria-label", `${unread} unread messages`);
      item.append(badge);
    }
    item.append(status);
    contactList.append(item);
  }
  contactStatus.textContent = statusMessage || latestMessageNotice || (contacts.length
    ? `${contacts.length} ${contacts.length === 1 ? "contact" : "contacts"} · ${onlineCount} online.`
    : "No other users are registered yet.");
  contactStatus.hidden = false;
}

function loadContacts() {
  if (!authenticated) return Promise.resolve();
  if (contactsRequest) return contactsRequest;
  const requestGeneration = authGeneration;
  contactStatus.textContent = "Loading contacts…";
  let request;
  request = (async () => {
    try {
      const response = await fetch("/api/contacts", {
        method: "GET",
        credentials: "same-origin",
        headers: { Accept: "application/json" },
      });
      const result = await readJSON(response);
      if (!authenticated || requestGeneration !== authGeneration) return;
      if (!response.ok || !Array.isArray(result.contacts)) {
        throw new Error("Unable to load contacts.");
      }
      contacts = result.contacts.map((contact) => ({
        id: Number(contact.id),
        username: String(contact.username || "Unknown user"),
        online: Boolean(contact.online),
        conversation_id: Number(contact.conversation_id) || 0,
        unread_count: Number(contact.unread_count) || 0,
        last_message_at: contact.last_message_at || null,
      }));
      for (const contact of contacts) {
        if (contact.unread_count > 0) unreadMessages.set(String(contact.id), contact.unread_count);
        else unreadMessages.delete(String(contact.id));
      }
      refreshMessageNotice();
      if (presenceSnapshotReceived) {
        for (const contact of contacts) {
          contact.online = onlineUsers.has(String(contact.id));
        }
      } else {
        onlineUsers.clear();
        for (const contact of contacts) {
          if (contact.online) onlineUsers.set(String(contact.id), { username: contact.username });
        }
      }
      renderContacts();
    } catch {
      if (!authenticated || requestGeneration !== authGeneration) return;
      contacts = [];
      renderContacts("Unable to load contacts. Select Feed and try again.");
    }
  })().finally(() => {
    if (contactsRequest !== request) return;
    contactsRequest = null;
    if (authenticated && requestGeneration !== authGeneration) {
      void loadContacts();
    } else if (authenticated && contactsRefreshPending) {
      contactsRefreshPending = false;
      void loadContacts();
    }
  });
  contactsRequest = request;
  return request;
}

function openConversation(contactButton) {
  if (!authenticated) return;
  const contactID = String(contactButton.dataset.contactId || "");
  const existing = chatWindows.get(contactID);
  if (existing) {
    if (existing.minimizedButton) {
      existing.minimizedButton.remove();
      existing.minimizedButton = null;
    }
    existing.element.hidden = false;
    existing.element.style.zIndex = String(++chatWindowSequence);
    chatLauncher.hidden = true;
    if (existing.conversationID > 0) void markConversationRead(existing.conversationID);
    return;
  }

  const name = String(contactButton.dataset.contactUsername || "Contact");
  const conversationID = Number(contactButton.dataset.conversationId) || 0;
  const element = chatWindowTemplate.content.firstElementChild.cloneNode(true);
  const title = element.querySelector("[data-chat-title]");
  const avatar = element.querySelector(".conversation-avatar");
  const status = element.querySelector("[data-chat-status]");
  const history = element.querySelector("[data-chat-history]");
  const retry = element.querySelector("[data-history-retry]");
  const form = element.querySelector("[data-message-form]");
  const body = element.querySelector("[data-message-body]");
  const image = element.querySelector('input[name="image"]');
  const messageStatus = element.querySelector("[data-message-status]");
  const suffix = `${contactID}-${++chatWindowSequence}`;
  body.id = `message-body-${suffix}`;
  form.querySelector('label[for="message-body"]').htmlFor = body.id;
  image.id = `message-image-${suffix}`;
  form.querySelector("label:not([for])").htmlFor = image.id;
  title.textContent = name;
  avatar.textContent = name.slice(0, 1).toUpperCase() || "?";

  const windowState = {
    contactID, conversationID, name, element, status, history, retry, form, body, image,
    messageStatus, messages: [], hasOlder: false, loading: false, generation: 1,
    request: null, scrollTimer: null, minimizedButton: null,
  };
  chatWindows.set(contactID, windowState);
  element.style.zIndex = String(chatWindowSequence);
  chatWindowsElement.append(element);
  chatLauncher.hidden = true;

  element.querySelector("[data-chat-close]").addEventListener("click", () => closeConversation(windowState));
  element.querySelector("[data-chat-minimize]").addEventListener("click", () => minimizeConversation(windowState));
  history.addEventListener("scroll", () => {
    if (windowState.scrollTimer !== null) return;
    windowState.scrollTimer = setTimeout(() => {
      windowState.scrollTimer = null;
      if (history.scrollHeight > history.clientHeight && history.scrollTop <= 64 &&
          windowState.hasOlder && !windowState.loading) {
        void loadConversationHistory(windowState, true);
      }
    }, 120);
  });
  retry.addEventListener("click", () => {
    void loadConversationHistory(windowState, retry.dataset.older === "true");
  });
  body.addEventListener("keydown", (event) => {
    if (event.key !== "Enter" || event.shiftKey || event.isComposing) return;
    event.preventDefault();
    form.requestSubmit();
  });
  form.addEventListener("input", () => {
    if (windowState.body.value.trim() || windowState.image.files.length > 0) {
      windowState.messageStatus.textContent = "";
    }
  });
  form.addEventListener("submit", (event) => void sendConversationMessage(windowState, event));

  if (conversationID <= 0) {
    status.textContent = "No messages in this conversation yet.";
    return;
  }
  status.textContent = "Loading messages…";
  void loadConversationHistory(windowState, false);
}

function closeConversation(windowState) {
  windowState.generation += 1;
  if (windowState.scrollTimer !== null) clearTimeout(windowState.scrollTimer);
  chatWindows.delete(windowState.contactID);
  windowState.element.remove();
  windowState.minimizedButton?.remove();
  windowState.minimizedButton = null;
  const anyVisible = Array.from(chatWindows.values()).some((item) => !item.element.hidden);
  if (!anyVisible) chatLauncher.hidden = !authenticated;
}

function minimizeConversation(windowState) {
  if (windowState.minimizedButton) return;
  windowState.element.hidden = true;
  const bubble = document.createElement("button");
  bubble.type = "button";
  bubble.className = "chat-minimized";
  bubble.setAttribute("aria-label", `Restore chat with ${windowState.name}`);
  bubble.title = `Restore chat with ${windowState.name}`;
  const avatar = document.createElement("span");
  avatar.className = "chat-minimized-avatar";
  avatar.setAttribute("aria-hidden", "true");
  avatar.textContent = windowState.name.slice(0, 1).toUpperCase() || "?";
  bubble.append(avatar, document.createTextNode(windowState.name));
  bubble.addEventListener("click", () => {
    windowState.minimizedButton.remove();
    windowState.minimizedButton = null;
    windowState.element.hidden = false;
    windowState.element.style.zIndex = String(++chatWindowSequence);
    if (windowState.conversationID > 0) void markConversationRead(windowState.conversationID);
  });
  windowState.minimizedButton = bubble;
  chatWindowsElement.append(bubble);
}

function refreshMessageNotice() {
  const unreadContact = contacts.find((contact) => (unreadMessages.get(String(contact.id)) || 0) > 0);
  latestMessageNotice = unreadContact ? `You have unread messages from ${unreadContact.username}.` : "";
}

async function markConversationRead(conversationID) {
  try {
    const response = await fetch(`/api/conversations/${encodeURIComponent(conversationID)}/read`, {
      method: "POST",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    if (!response.ok) throw new Error("Unable to mark messages as read.");
    const contact = contacts.find((item) => Number(item.conversation_id) === Number(conversationID));
    if (contact) unreadMessages.delete(String(contact.id));
    refreshMessageNotice();
    renderContacts();
    if (contactsRequest) contactsRefreshPending = true;
    await loadContacts();
  } catch {
    void loadContacts();
  }
}

function renderConversationMessages(windowState) {
  windowState.history.replaceChildren();
  for (const message of windowState.messages) {
    rememberMessageID(message.id);
    const article = document.createElement("article");
    article.className = "chat-message";
    if (Number(message.sender_id) === currentUserID) article.classList.add("chat-message-own");
    article.dataset.messageId = String(message.id);
    const metadata = document.createElement("p");
    metadata.className = "chat-message-meta";
    const sender = document.createElement("span");
    sender.textContent = String(message.sender || "Unknown user");
    const sentAt = document.createElement("time");
    sentAt.dateTime = String(message.created_at);
    sentAt.textContent = formatPostDate(message.created_at);
    const body = document.createElement("p");
    body.textContent = String(message.body);
    metadata.append(sender, sentAt);
    article.append(metadata, body);
    if (typeof message.image_url === "string" && message.image_url.startsWith("/api/messages/")) {
      const image = document.createElement("img");
      image.className = "chat-message-image";
      image.src = message.image_url;
      image.alt = `Image attached by ${String(message.sender || "a contact")}`;
      image.loading = "lazy";
      article.append(image);
    }
    windowState.history.append(article);
  }
}

function rememberMessageID(id) {
  const key = String(id);
  if (seenMessageIDs.has(key)) return;
  if (seenMessageIDs.size >= 2000) seenMessageIDs.delete(seenMessageIDs.values().next().value);
  seenMessageIDs.add(key);
}

function loadConversationHistory(windowState, older) {
  if (!authenticated || windowState.conversationID <= 0) return Promise.resolve();
  if (windowState.request) return windowState.request;
  if (older && (!windowState.hasOlder || windowState.loading)) return Promise.resolve();
  const requestGeneration = windowState.generation;
  const conversationID = windowState.conversationID;
  const cursor = older ? windowState.messages[0] : null;
  if (older && !cursor) return Promise.resolve();
  windowState.loading = true;
  windowState.retry.hidden = true;
  windowState.status.textContent = older ? "Loading earlier messages…" : "Loading messages…";

  const url = new URL(`/api/conversations/${encodeURIComponent(conversationID)}/messages`, window.location.origin);
  if (older) {
    url.searchParams.set("before_id", String(cursor.id));
    url.searchParams.set("before_created_at", String(cursor.created_at));
  }

  let request;
  request = (async () => {
    try {
      const response = await fetch(url, {
        method: "GET",
        credentials: "same-origin",
        headers: { Accept: "application/json" },
      });
      const result = await readJSON(response);
      if (!authenticated || requestGeneration !== windowState.generation || !chatWindows.has(windowState.contactID)) return;
      if (!response.ok || !Array.isArray(result.messages)) {
        throw new Error(result.error || "Unable to load messages.");
      }

      if (older) {
        if (result.messages.length === 0) {
          windowState.hasOlder = false;
          windowState.status.textContent = "You’re at the beginning of this conversation.";
          return;
        }
        const previousHeight = windowState.history.scrollHeight;
        const previousTop = windowState.history.scrollTop;
        const existingIDs = new Set(windowState.messages.map((message) => String(message.id)));
        const olderMessages = result.messages.filter((message) => !existingIDs.has(String(message.id)));
        windowState.messages = [...olderMessages, ...windowState.messages];
        windowState.hasOlder = Boolean(result.has_older);
        renderConversationMessages(windowState);
        windowState.history.scrollTop = previousTop + (windowState.history.scrollHeight - previousHeight);
        windowState.status.textContent = "";
      } else {
        const byID = new Map();
        for (const message of result.messages) byID.set(String(message.id), message);
        for (const message of windowState.messages) byID.set(String(message.id), message);
        windowState.messages = Array.from(byID.values()).sort((a, b) =>
          a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
        windowState.hasOlder = Boolean(result.has_older);
        renderConversationMessages(windowState);
        windowState.history.scrollTop = windowState.history.scrollHeight;
        windowState.status.textContent = windowState.messages.length ? "" : "No messages in this conversation yet.";
        void markConversationRead(conversationID);
      }
    } catch {
      if (!authenticated || requestGeneration !== windowState.generation || !chatWindows.has(windowState.contactID)) return;
      windowState.status.textContent = older
        ? "Unable to load earlier messages. Try again."
        : "Unable to load this conversation. Try again.";
      windowState.retry.dataset.older = String(older);
      windowState.retry.hidden = false;
    }
  })().finally(() => {
    if (windowState.request === request) {
      windowState.request = null;
      windowState.loading = false;
    }
  });
  windowState.request = request;
  return request;
}

function applyIncomingMessage(message) {
  if (!message || !Number.isSafeInteger(message.id) || !Number.isSafeInteger(message.conversation_id) ||
      !Number.isSafeInteger(message.sender_id) || typeof message.body !== "string" ||
      typeof message.created_at !== "string") return;
  if (seenMessageIDs.has(String(message.id))) return;
  rememberMessageID(message.id);
  const conversationID = message.conversation_id;
  const windowState = Array.from(chatWindows.values()).find((item) => item.conversationID === conversationID);
  if (windowState) {
    const wasNearBottom = windowState.history.scrollHeight - windowState.history.scrollTop - windowState.history.clientHeight < 80;
    if (!windowState.messages.some((item) => String(item.id) === String(message.id))) {
      windowState.messages.push(message);
      windowState.messages.sort((a, b) => a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
      renderConversationMessages(windowState);
      if (wasNearBottom) windowState.history.scrollTop = windowState.history.scrollHeight;
    }
    void markConversationRead(conversationID);
    const senderName = String(message.sender || "A contact");
    windowState.status.textContent = wasNearBottom
      ? `New message from ${senderName}.`
      : `New message from ${senderName}. Scroll down to read it.`;
  } else {
    const senderIsContact = contacts.some((contact) => String(contact.id) === String(message.sender_id));
    if (senderIsContact) {
      const senderName = String(message.sender || "A contact");
      const contact = contacts.find((item) => String(item.id) === String(message.sender_id));
      if (contact) unreadMessages.set(String(contact.id), (unreadMessages.get(String(contact.id)) || 0) + 1);
      latestMessageNotice = `New private message from ${senderName}.`;
    }
    renderContacts();
  }
  if (contactsRequest) contactsRefreshPending = true;
  else void loadContacts();
}

for (const toggle of chatToggles) {
  toggle.addEventListener("click", () => {
    const expandedWindows = Array.from(chatWindows.values()).filter((item) => !item.minimizedButton);
    if (expandedWindows.length === 0 && chatWindows.size === 0) {
      const firstContact = contactList.querySelector("[data-open-contact]");
      if (firstContact) {
        firstContact.focus();
        firstContact.scrollIntoView({ behavior: "smooth", block: "center" });
      }
      return;
    }
    const show = expandedWindows.some((item) => item.element.hidden);
    for (const windowState of expandedWindows) {
      windowState.element.hidden = !show;
      if (show && windowState.conversationID > 0) void markConversationRead(windowState.conversationID);
    }
    chatLauncher.hidden = chatWindows.size > 0 && show ? true : !authenticated;
  });
}

async function sendConversationMessage(windowState, event) {
  event.preventDefault();
  if (!authenticated) return;
  if (!windowState.body.value.trim() && windowState.image.files.length === 0) {
    return;
  }
  const requestGeneration = authGeneration;
  const submitButton = windowState.form.querySelector('button[type="submit"]');
  submitButton.disabled = true;
  windowState.messageStatus.textContent = "Sending…";
  try {
    const response = await fetch("/api/messages", {
      method: "POST",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
      },
      body: (() => {
        const form = new FormData(windowState.form);
        form.set("recipient_id", windowState.contactID);
        return form;
      })(),
    });
    const result = await readJSON(response);
    if (!authenticated || requestGeneration !== authGeneration) return;
    if (!response.ok || !result.message) throw new Error(result.error || "Unable to send message.");
    const message = result.message;
    windowState.conversationID = Number(message.conversation_id);
    if (!windowState.messages.some((item) => String(item.id) === String(message.id))) windowState.messages.push(message);
    windowState.messages.sort((a, b) => a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
    renderConversationMessages(windowState);
    windowState.history.scrollTop = windowState.history.scrollHeight;
    windowState.status.textContent = "";
    windowState.body.value = "";
    windowState.image.value = "";
    windowState.messageStatus.textContent = "";
    latestMessageNotice = "";
    void loadContacts();
  } catch (error) {
    if (authenticated && requestGeneration === authGeneration) {
      windowState.messageStatus.textContent = error.message || "Unable to send message. Try again.";
    }
  } finally {
    submitButton.disabled = false;
  }
}

profileForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!authenticated) return;
  const requestGeneration = authGeneration;
  const submitButton = profileForm.querySelector('button[type="submit"]');
  submitButton.disabled = true;
  profileStatus.textContent = "Saving profile…";
  try {
    const response = await fetch("/api/profile", {
      method: "PUT",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      },
      body: new URLSearchParams(new FormData(profileForm)),
    });
    const result = await readJSON(response);
    if (!authenticated || requestGeneration !== authGeneration) return;
    if (!response.ok) throw new Error(result.error || "Unable to save profile.");
    profileStatus.textContent = "Profile saved.";
    await loadProfile(0, true);
  } catch (error) {
    if (authenticated && requestGeneration === authGeneration) {
      profileStatus.textContent = error.message || "Unable to save profile.";
    }
  } finally {
    submitButton.disabled = false;
  }
});

function applyPresenceEvent(event) {
  if (event.type === "presence.snapshot") {
    const users = event.payload.online_users;
    if (!Array.isArray(users)) return;
    onlineUsers.clear();
    presenceSnapshotReceived = true;
    for (const user of users) {
      if (Number.isSafeInteger(user?.user_id) && user.user_id > 0 && typeof user.username === "string") {
        onlineUsers.set(String(user.user_id), { user_id: user.user_id, username: user.username });
      }
    }
    for (const contact of contacts) {
      contact.online = onlineUsers.has(String(contact.id));
    }
    renderContacts();
    return;
  }
  if (event.type === "presence.changed") {
    const user = event.payload;
    if (!Number.isSafeInteger(user.user_id) || user.user_id <= 0 ||
        typeof user.username !== "string" || typeof user.online !== "boolean") return;
    if (user.online) {
      onlineUsers.set(String(user.user_id), { user_id: user.user_id, username: user.username });
    } else {
      onlineUsers.delete(String(user.user_id));
    }
    const contact = contacts.find((item) => String(item.id) === String(user.user_id));
    if (!contact && user.online) {
      if (contactsRequest) contactsRefreshPending = true;
      else void loadContacts();
      return;
    }
    if (contact) contact.online = user.online;
    renderContacts();
  }
}

document.addEventListener("forum:realtime-event", (event) => {
  if (!authenticated) return;
  if (event.detail?.type === "message.new") {
    applyIncomingMessage(event.detail.payload?.message);
    return;
  }
  applyPresenceEvent(event.detail);
});

function closeRealtimeSocket() {
  if (realtimeRetryTimer !== null) {
    clearTimeout(realtimeRetryTimer);
    realtimeRetryTimer = null;
  }
  if (realtimeStableTimer !== null) {
    clearTimeout(realtimeStableTimer);
    realtimeStableTimer = null;
  }
  const socket = realtimeSocket;
  realtimeSocket = null;
  realtimeRetryCount = 0;
  if (socket && socket.readyState < WebSocket.CLOSING) {
    socket.close(1000, "User logged out");
  }
  setRealtimeStatus("Live connection closed.");
}

function connectRealtimeSocket() {
  if (!authenticated || realtimeSocket || realtimeRetryTimer !== null) return;
  if (realtimeRetryCount >= maxRealtimeRetries) {
    setRealtimeStatus("Live connection is unavailable. You can retry.", true);
    return;
  }

  const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
  const socket = new WebSocket(`${scheme}//${window.location.host}/ws`);
  realtimeSocket = socket;
  setRealtimeStatus("Connecting to live services…");

  socket.addEventListener("open", () => {
    if (realtimeSocket !== socket) return;
    realtimeStableTimer = setTimeout(() => {
      if (realtimeSocket === socket) realtimeRetryCount = 0;
      realtimeStableTimer = null;
    }, 30000);
    setRealtimeStatus("Live connection connected.");
    contactStatus.textContent = "Connected. Updating presence…";
    if (contactsRequest) contactsRefreshPending = true;
    else void loadContacts();
  });

  socket.addEventListener("message", (event) => {
    if (realtimeSocket !== socket || typeof event.data !== "string") return;
    try {
      const envelope = JSON.parse(event.data);
      if (!envelope || typeof envelope.type !== "string" ||
          !envelope.payload || typeof envelope.payload !== "object" ||
          Array.isArray(envelope.payload)) return;
      document.dispatchEvent(new CustomEvent("forum:realtime-event", { detail: envelope }));
    } catch {
      // Ignore malformed server frames; the connection remains available.
    }
  });

  socket.addEventListener("error", () => {
    if (realtimeSocket === socket) setRealtimeStatus("Live connection interrupted…");
  });

  socket.addEventListener("close", (event) => {
    if (realtimeSocket !== socket) return;
    realtimeSocket = null;
    if (realtimeStableTimer !== null) {
      clearTimeout(realtimeStableTimer);
      realtimeStableTimer = null;
    }
    if (!authenticated) {
      setRealtimeStatus("Live connection closed.");
      return;
    }
    if (event.code === 1008) {
      setAuthenticated(false);
      showView("login");
      showAuthStatus("Your session ended. Please sign in again.", "error");
      return;
    }
    clearOnlineUsers("Presence is unavailable while disconnected.");
    void loadContacts();
    if (realtimeRetryCount >= maxRealtimeRetries) {
      setRealtimeStatus("Live connection is unavailable. You can retry.", true);
      return;
    }
    realtimeRetryCount += 1;
    const delay = Math.min(1000 * (2 ** (realtimeRetryCount - 1)), 8000);
    setRealtimeStatus(`Live connection lost. Retrying in ${Math.ceil(delay / 1000)}s…`);
    realtimeRetryTimer = setTimeout(() => {
      realtimeRetryTimer = null;
      connectRealtimeSocket();
    }, delay);
  });
}

realtimeRetry.addEventListener("click", () => {
  realtimeRetryCount = 0;
  setRealtimeStatus("Reconnecting to live services…");
  connectRealtimeSocket();
});

function showAuthStatus(message, status) {
  authStatus.textContent = message;
  authStatus.dataset.status = status;
  authStatus.hidden = !message;
}

async function readJSON(response) {
  try {
    return await response.json();
  } catch {
    throw new Error("The server returned an unreadable response.");
  }
}

document.addEventListener("click", (event) => {
  if (event.target.closest("[data-home-feed]")) {
    showView(authenticated ? "feed" : "login");
    if (authenticated) void loadFeed();
    return;
  }
  const profileButton = event.target.closest("[data-open-profile]");
  if (profileButton) {
    void loadProfile(Number(profileButton.dataset.openProfile), false);
    return;
  }
  const contactButton = event.target.closest("[data-open-contact]");
  if (contactButton) {
    void openConversation(contactButton);
    return;
  }
  const postButton = event.target.closest("[data-open-post]");
  if (postButton) {
    void loadPostDetail(postButton.dataset.openPost);
    return;
  }
  const button = event.target.closest("[data-view]");

  if (event.target.closest("[data-chat-toggle]")) return;

  if (!button) {
    return;
  }

  showView(button.dataset.view);

  if (button.dataset.view === "create-post") {
    void loadCategories();
  }

  if (button.dataset.view === "feed" && authenticated) {
    void loadFeed();
  }

  if (button.hasAttribute("data-profile-owner") && authenticated) {
    void loadProfile(0, true);
  }
});

async function loadCategories() {
  if (!authenticated || categoriesLoaded || categoriesLoading) {
    return;
  }

  categoriesLoading = true;
  const requestGeneration = authGeneration;
  postCategories.textContent = "Loading categories…";
  postSubmitButton.disabled = true;

  try {
    const response = await fetch("/api/categories", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);

    if (!authenticated || requestGeneration !== authGeneration) {
      return;
    }

    if (!response.ok || !Array.isArray(result.categories)) {
      throw new Error("Unable to load categories.");
    }

    postCategories.replaceChildren();
    if (result.categories.length === 0) {
      postCategories.textContent = "No categories are available.";
      postResult.textContent = "No categories are available for posts.";
      postResult.dataset.status = "error";
      postResult.hidden = false;
      return;
    }

    const options = document.createElement("div");
    options.className = "category-options";
    for (const category of result.categories) {
      const label = document.createElement("label");
      const checkbox = document.createElement("input");
      checkbox.type = "checkbox";
      checkbox.name = "category";
      checkbox.value = String(category.id);
      checkbox.id = `post-category-${category.id}`;

      label.className = "category-option";
      label.htmlFor = checkbox.id;
      label.append(checkbox, document.createTextNode(String(category.name)));
      options.append(label);
    }
    postCategories.append(options);
    postResult.hidden = true;
    postResult.textContent = "";
    categoriesLoaded = true;
    postSubmitButton.disabled = false;
  } catch {
    if (!authenticated || requestGeneration !== authGeneration) {
      return;
    }
    postCategories.textContent = "Unable to load categories. Select Create Post to try again.";
    postResult.textContent = "Unable to load categories. Please try again.";
    postResult.dataset.status = "error";
    postResult.hidden = false;
  } finally {
    categoriesLoading = false;
    if (authenticated && requestGeneration !== authGeneration) {
      void loadCategories();
    }
  }
}

function formatPostDate(value) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? String(value) : date.toLocaleString();
}

async function loadProfile(profileID, owner) {
  if (!authenticated) return;
  const requestGeneration = authGeneration;
  profileTitle.textContent = owner ? "My Profile" : "Profile";
  profileStatus.textContent = "Loading profile…";
  profileDetails.replaceChildren();
  profileForm.hidden = true;
  showView("profile");
  const url = owner ? "/api/profile" : `/api/users/${encodeURIComponent(profileID)}/profile`;
  try {
    const response = await fetch(url, { credentials: "same-origin", headers: { Accept: "application/json" } });
    const result = await readJSON(response);
    if (!authenticated || requestGeneration !== authGeneration) return;
    if (!response.ok || !result.profile) throw new Error(result.error || "Unable to load profile.");
    const profile = result.profile;
    profileTitle.textContent = owner ? "My Profile" : `${profile.username}’s Profile`;
    profileStatus.textContent = owner
      ? "Your public profile. Only you can edit these details."
      : "Public profile · read-only.";
    const fields = [
      ["Nickname", profile.username], ["First name", profile.first_name], ["Last name", profile.last_name],
      ["Age", String(profile.age)], ["Gender", profile.gender],
    ];
    profileDetails.replaceChildren();
    for (const [label, value] of fields) {
      const term = document.createElement("dt");
      term.textContent = label;
      const detail = document.createElement("dd");
      detail.textContent = String(value);
      profileDetails.append(term, detail);
    }
    if (owner) {
      profileForm.elements.namedItem("first_name").value = profile.first_name;
      profileForm.elements.namedItem("last_name").value = profile.last_name;
      profileForm.elements.namedItem("age").value = String(profile.age);
      profileForm.elements.namedItem("gender").value = profile.gender;
    }
    profileForm.hidden = !owner;
    profileStatus.textContent = owner ? "Your public profile. Email and account credentials are private." : "Public profile.";
  } catch (error) {
    if (!authenticated || requestGeneration !== authGeneration) return;
    profileStatus.textContent = error.message || "Unable to load profile.";
  }
}

function renderFeed(posts) {
  feedPosts.replaceChildren();

  for (const post of posts) {
    const article = document.createElement("article");
    article.className = "feed-post";

    const title = document.createElement("button");
    title.type = "button";
    title.className = "feed-post-title";
    title.dataset.openPost = String(post.id);
    title.textContent = String(post.title);
    article.append(title);

    const metadata = document.createElement("p");
    metadata.className = "feed-post-meta";
    metadata.textContent = `${String(post.author?.username || "Unknown author")} · ${formatPostDate(post.created_at)}`;
    article.append(metadata);

    const body = document.createElement("p");
    body.className = "feed-post-body";
    body.textContent = String(post.body);
    article.append(body);

    if (Array.isArray(post.categories) && post.categories.length > 0) {
      const categories = document.createElement("div");
      categories.className = "feed-post-categories";
      for (const category of post.categories) {
        const tag = document.createElement("span");
        tag.className = "feed-category-tag";
        tag.textContent = String(category.name);
        categories.append(tag);
      }
      article.append(categories);
    }

    const reactions = document.createElement("p");
    reactions.className = "feed-post-reactions";
    reactions.textContent = `Likes: ${Number(post.likes) || 0} · Dislikes: ${Number(post.dislikes) || 0}`;
    article.append(reactions);

    feedPosts.append(article);
  }
}

function setStatus(element, message) {
  element.textContent = message;
  element.hidden = !message;
}

function renderPostDetail(post) {
  detailPost.replaceChildren();
  const metadata = document.createElement("p");
  metadata.className = "feed-post-meta";
  metadata.textContent = `${String(post.author?.username || "Unknown author")} · ${formatPostDate(post.created_at)}`;
  const body = document.createElement("p");
  body.className = "feed-post-body";
  body.textContent = String(post.body);
  detailPost.append(metadata, body);

  const categories = document.createElement("div");
  categories.className = "feed-post-categories";
  for (const category of post.categories || []) {
    const tag = document.createElement("span");
    tag.className = "feed-category-tag";
    tag.textContent = String(category.name);
    categories.append(tag);
  }
  if (categories.childElementCount) detailPost.append(categories);

  postComments.replaceChildren();
  if (!post.comments?.length) {
    setStatus(commentsStatus, "No comments yet.");
  } else {
    setStatus(commentsStatus, "");
    for (const comment of post.comments) {
      const article = document.createElement("article");
      article.className = "post-comment";
      const content = document.createElement("p");
      content.textContent = String(comment.body);
      const byline = document.createElement("p");
      byline.className = "feed-post-meta";
      byline.textContent = `${String(comment.author?.username || "Unknown author")} · ${formatPostDate(comment.created_at)}`;
      article.append(content, byline);
      postComments.append(article);
    }
  }
}

async function loadPostDetail(postID) {
  if (!authenticated || !/^\d+$/.test(String(postID))) return;
  selectedPostID = String(postID);
  detailPost.replaceChildren();
  postComments.replaceChildren();
  commentForm.hidden = !authenticated;
  commentForm.action = `/api/posts/${encodeURIComponent(selectedPostID)}/comments`;
  commentResult.hidden = true;
  setStatus(detailStatus, "Loading post and comments…");
  setStatus(commentsStatus, "");
  showView("post");
  try {
    const response = await fetch(`/api/posts/${encodeURIComponent(selectedPostID)}`, {
      credentials: "same-origin", headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);
    if (!authenticated || selectedPostID !== String(postID)) return;
    if (!response.ok) throw new Error(result.error || "Unable to load this post.");
    document.querySelector("#detail-title").textContent = String(result.title || "Post detail");
    renderPostDetail(result);
    setStatus(detailStatus, "");
  } catch (error) {
    if (!authenticated || selectedPostID !== String(postID)) return;
    setStatus(detailStatus, error.message || "Unable to load this post. Please try again.");
  }
}

function loadFeed() {
  if (!authenticated) {
    return Promise.resolve();
  }
  if (feedRequest) {
    return feedRequest;
  }

  feedStatus.textContent = "Loading posts…";
  feedStatus.hidden = false;
  feedPosts.replaceChildren();
  const requestGeneration = authGeneration;

  feedRequest = (async () => {
    try {
      const response = await fetch("/api/posts", {
        method: "GET",
        credentials: "same-origin",
        headers: { Accept: "application/json" },
      });
      const result = await readJSON(response);

      if (!authenticated || requestGeneration !== authGeneration) {
        return;
      }
      if (!response.ok || !Array.isArray(result.posts)) {
        throw new Error("Unable to load posts.");
      }

      if (result.posts.length === 0) {
        feedPosts.replaceChildren();
        feedStatus.textContent = "No posts yet.";
        feedStatus.hidden = false;
        return;
      }

      renderFeed(result.posts);
      feedStatus.textContent = "";
      feedStatus.hidden = true;
    } catch {
      if (!authenticated || requestGeneration !== authGeneration) {
        return;
      }
      feedPosts.replaceChildren();
      feedStatus.textContent = "Unable to load posts. Select Feed to try again.";
      feedStatus.hidden = false;
    }
  })().finally(() => {
    feedRequest = null;
    if (authenticated && requestGeneration !== authGeneration) {
      void loadFeed();
    }
  });

  return feedRequest;
}

async function refreshFeed() {
  if (feedRequest) {
    await feedRequest;
  }
  return loadFeed();
}

registrationForm.addEventListener("submit", async (event) => {
  event.preventDefault();

  registrationResult.hidden = true;
  registrationResult.textContent = "";

  const submitButton = registrationForm.querySelector('button[type="submit"]');
  submitButton.disabled = true;

  try {
    const response = await fetch(registrationForm.action, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      },
      body: new URLSearchParams(new FormData(registrationForm)),
    });

    const result = await readJSON(response);

    registrationResult.textContent = response.ok
      ? result.message || "Registration successful. Please log in."
      : result.error || "Registration failed. Please check your details.";
    registrationResult.dataset.status = response.ok ? "success" : "error";
    registrationResult.hidden = false;

    if (response.ok) {
      registrationForm.reset();
    }
  } catch {
    registrationResult.textContent = "Unable to complete registration. Please try again.";
    registrationResult.dataset.status = "error";
    registrationResult.hidden = false;
  } finally {
    submitButton.disabled = false;
  }
});

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  loginResult.hidden = true;
  loginResult.textContent = "";
  showAuthStatus("", "");

  const submitButton = loginForm.querySelector('button[type="submit"]');
  submitButton.disabled = true;

  try {
    const response = await fetch(loginForm.action, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      },
      body: new URLSearchParams(new FormData(loginForm)),
    });
    const result = await readJSON(response);

    if (!response.ok) {
      loginResult.textContent = result.error || "Login failed. Please check your details.";
      loginResult.dataset.status = "error";
      loginResult.hidden = false;
      return;
    }

    loginForm.reset();
    setAuthenticated(true);
    void loadCurrentUser();
    showView("feed");
    void loadFeed();
    showAuthStatus(result.message || "Login successful.", "success");
  } catch {
    loginResult.textContent = "Unable to log in. Please try again.";
    loginResult.dataset.status = "error";
    loginResult.hidden = false;
  } finally {
    submitButton.disabled = false;
  }
});

postForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  const requestGeneration = authGeneration;
  postResult.hidden = true;
  postResult.textContent = "";
  postSubmitButton.disabled = true;

  try {
    const response = await fetch(postForm.action, {
      method: "POST",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      },
      body: new URLSearchParams(new FormData(postForm)),
    });
    const result = await readJSON(response);

    if (!authenticated || requestGeneration !== authGeneration) {
      return;
    }

    postResult.textContent = response.ok
      ? `${result.message || "Post created."} ID: ${result.post_id}.`
      : result.error || "Unable to create post. Please check the title, body, and categories.";
    postResult.dataset.status = response.ok ? "success" : "error";
    postResult.hidden = false;

    if (response.ok) {
      postForm.reset();
      showView("feed");
      showAuthStatus(result.message || "Post created.", "success");
      await refreshFeed();
    }
  } catch {
    if (!authenticated || requestGeneration !== authGeneration) {
      return;
    }
    postResult.textContent = "Unable to create post. Please try again.";
    postResult.dataset.status = "error";
    postResult.hidden = false;
  } finally {
    postSubmitButton.disabled = !categoriesLoaded;
  }
});

commentForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!authenticated || !selectedPostID) return;
  const submit = commentForm.querySelector('button[type="submit"]');
  submit.disabled = true;
  commentResult.hidden = true;
  try {
    const response = await fetch(commentForm.action, {
      method: "POST", credentials: "same-origin",
      headers: { Accept: "application/json", "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8" },
      body: new URLSearchParams(new FormData(commentForm)),
    });
    const result = await readJSON(response);
    if (!authenticated) return;
    if (!response.ok) throw new Error(result.error || "Unable to add comment.");
    commentForm.reset();
    commentResult.textContent = result.message || "Comment added.";
    commentResult.dataset.status = "success";
    commentResult.hidden = false;
    await loadPostDetail(selectedPostID);
  } catch (error) {
    commentResult.textContent = error.message || "Unable to add comment. Please try again.";
    commentResult.dataset.status = "error";
    commentResult.hidden = false;
  } finally {
    submit.disabled = false;
  }
});

logoutButton.addEventListener("click", async () => {
  logoutButton.disabled = true;
  showAuthStatus("", "");

  try {
    const response = await fetch("/logout", {
      method: "POST",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);

    if (!response.ok) {
      showAuthStatus(result.error || "Unable to log out. Please try again.", "error");
      return;
    }

    setAuthenticated(false);
    showView("login");
    showAuthStatus(result.message || "Logged out.", "success");
  } catch {
    showAuthStatus("Unable to log out. Please try again.", "error");
  } finally {
    logoutButton.disabled = false;
  }
});

async function restoreSession() {
  try {
    const response = await fetch("/api/session", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);

    if (!response.ok) {
      throw new Error("Session status request failed.");
    }

    setAuthenticated(Boolean(result.authenticated));
    showView(authenticated ? "feed" : "login");
    if (authenticated) {
      void loadCurrentUser();
      void loadFeed();
    }
  } catch {
    setAuthenticated(false);
    showView("login");
    showAuthStatus("Unable to check your sign-in status. Please try again.", "error");
  }
}

async function loadCurrentUser() {
  try {
    const response = await fetch("/api/profile", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);
    if (response.ok && Number.isSafeInteger(result.profile?.id)) {
      currentUserID = result.profile.id;
      for (const windowState of chatWindows.values()) renderConversationMessages(windowState);
    }
  } catch {
    currentUserID = 0;
  }
}

setAuthenticated(false);
showView("login");
restoreSession();
