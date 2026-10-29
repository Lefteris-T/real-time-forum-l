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
const chatTitle = document.querySelector("#chat-title");
const chatStatus = document.querySelector("#chat-status");
const chatHistory = document.querySelector("#chat-history");
const historyRetry = document.querySelector("#history-retry");
const messageForm = document.querySelector("#message-form");
const messageBody = document.querySelector("#message-body");
const messageStatus = document.querySelector("#message-status");
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
let selectedConversationID = 0;
let selectedConversationName = "";
let historyMessages = [];
let historyHasOlder = false;
let historyLoading = false;
let historyGeneration = 0;
let historyRequest = null;
let historyScrollTimer = null;
const unreadMessages = new Map();
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
    unreadMessages.clear();
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
    selectedConversationID = 0;
    selectedConversationName = "";
    historyMessages = [];
    historyHasOlder = false;
    historyLoading = false;
    historyGeneration += 1;
    historyRequest = null;
    chatHistory.replaceChildren();
    historyRetry.hidden = true;
    chatTitle.textContent = "Chat";
    chatStatus.textContent = "Sign in to view conversations.";
    messageForm.hidden = true;
    messageForm.reset();
    messageStatus.textContent = "";
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
    const status = document.createElement("span");
    status.className = "contact-presence";
    status.textContent = contact.online ? "Online" : "Offline";
    item.append(dot, name);
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
  selectedConversationID = Number(contactButton.dataset.conversationId) || 0;
  selectedConversationName = String(contactButton.dataset.contactUsername || "Contact");
  const selectedContactID = String(contactButton.dataset.contactId || "");
  renderContacts();
  historyGeneration += 1;
  historyRequest = null;
  historyLoading = false;
  historyHasOlder = false;
  historyMessages = [];
  historyRetry.hidden = true;
  if (historyScrollTimer !== null) {
    clearTimeout(historyScrollTimer);
    historyScrollTimer = null;
  }
  chatTitle.textContent = selectedConversationName;
  messageForm.hidden = false;
  messageStatus.textContent = "";
  messageForm.dataset.recipientId = selectedContactID;
  chatHistory.replaceChildren();
  chatHistory.scrollTop = 0;
  showView("chat");

  if (selectedConversationID <= 0) {
    chatStatus.textContent = "No messages in this conversation yet.";
    return;
  }
  chatStatus.textContent = "Loading messages…";
  void loadConversationHistory(false);
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

function renderConversationMessages() {
  chatHistory.replaceChildren();
  for (const message of historyMessages) {
    const article = document.createElement("article");
    article.className = "chat-message";
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
    chatHistory.append(article);
  }
}

function loadConversationHistory(older) {
  if (!authenticated || selectedConversationID <= 0) return Promise.resolve();
  if (historyRequest) return historyRequest;
  if (older && (!historyHasOlder || historyLoading)) return Promise.resolve();
  const requestGeneration = historyGeneration;
  const conversationID = selectedConversationID;
  const cursor = older ? historyMessages[0] : null;
  if (older && !cursor) return Promise.resolve();
  historyLoading = true;
  historyRetry.hidden = true;
  chatStatus.textContent = older ? "Loading earlier messages…" : "Loading messages…";

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
      if (!authenticated || requestGeneration !== historyGeneration || conversationID !== selectedConversationID) return;
      if (!response.ok || !Array.isArray(result.messages)) {
        throw new Error(result.error || "Unable to load messages.");
      }

      if (older) {
        if (result.messages.length === 0) {
          historyHasOlder = false;
          chatStatus.textContent = "You’re at the beginning of this conversation.";
          return;
        }
        const previousHeight = chatHistory.scrollHeight;
        const previousTop = chatHistory.scrollTop;
        const existingIDs = new Set(historyMessages.map((message) => String(message.id)));
        const olderMessages = result.messages.filter((message) => !existingIDs.has(String(message.id)));
        historyMessages = [...olderMessages, ...historyMessages];
        historyHasOlder = Boolean(result.has_older);
        renderConversationMessages();
        chatHistory.scrollTop = previousTop + (chatHistory.scrollHeight - previousHeight);
        chatStatus.textContent = "";
      } else {
        const byID = new Map();
        for (const message of result.messages) byID.set(String(message.id), message);
        for (const message of historyMessages) byID.set(String(message.id), message);
        historyMessages = Array.from(byID.values()).sort((a, b) =>
          a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
        historyHasOlder = Boolean(result.has_older);
        renderConversationMessages();
        chatHistory.scrollTop = chatHistory.scrollHeight;
        chatStatus.textContent = historyMessages.length ? "" : "No messages in this conversation yet.";
        if (activeView === "chat") void markConversationRead(conversationID);
      }
    } catch {
      if (!authenticated || requestGeneration !== historyGeneration || conversationID !== selectedConversationID) return;
      chatStatus.textContent = older
        ? "Unable to load earlier messages. Try again."
        : "Unable to load this conversation. Try again.";
      historyRetry.dataset.older = String(older);
      historyRetry.hidden = false;
    }
  })().finally(() => {
    if (historyRequest === request) {
      historyRequest = null;
      historyLoading = false;
    }
  });
  historyRequest = request;
  return request;
}

function applyIncomingMessage(message) {
  if (!message || !Number.isSafeInteger(message.id) || !Number.isSafeInteger(message.conversation_id) ||
      !Number.isSafeInteger(message.sender_id) || typeof message.body !== "string" ||
      typeof message.created_at !== "string") return;
  const conversationID = message.conversation_id;
  const isOpen = activeView === "chat" && selectedConversationID === conversationID;
  const wasNearBottom = chatHistory.scrollHeight - chatHistory.scrollTop - chatHistory.clientHeight < 80;
  if (isOpen) {
    if (!historyMessages.some((item) => String(item.id) === String(message.id))) {
      historyMessages.push(message);
      historyMessages.sort((a, b) => a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
      renderConversationMessages();
      if (wasNearBottom) chatHistory.scrollTop = chatHistory.scrollHeight;
    }
    void markConversationRead(conversationID);
    const senderName = String(message.sender || "A contact");
    chatStatus.textContent = wasNearBottom
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

chatHistory.addEventListener("scroll", () => {
  if (historyScrollTimer !== null) return;
  historyScrollTimer = setTimeout(() => {
    historyScrollTimer = null;
    if (chatHistory.scrollHeight > chatHistory.clientHeight && chatHistory.scrollTop <= 64 &&
        historyHasOlder && !historyLoading) {
      void loadConversationHistory(true);
    }
  }, 120);
});

historyRetry.addEventListener("click", () => {
  void loadConversationHistory(historyRetry.dataset.older === "true");
});

messageForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  if (!authenticated || !messageForm.dataset.recipientId) return;
  const requestGeneration = authGeneration;
  const submitButton = messageForm.querySelector('button[type="submit"]');
  submitButton.disabled = true;
  messageStatus.textContent = "Sending…";
  try {
    const response = await fetch("/api/messages", {
      method: "POST",
      credentials: "same-origin",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/x-www-form-urlencoded;charset=UTF-8",
      },
      body: new URLSearchParams({ recipient_id: messageForm.dataset.recipientId, body: messageBody.value }),
    });
    const result = await readJSON(response);
    if (!authenticated || requestGeneration !== authGeneration) return;
    if (!response.ok || !result.message) throw new Error(result.error || "Unable to send message.");
    const message = result.message;
    selectedConversationID = Number(message.conversation_id);
    if (!historyMessages.some((item) => String(item.id) === String(message.id))) historyMessages.push(message);
    historyMessages.sort((a, b) => a.created_at.localeCompare(b.created_at) || Number(a.id) - Number(b.id));
    renderConversationMessages();
    chatHistory.scrollTop = chatHistory.scrollHeight;
    chatStatus.textContent = "";
    messageBody.value = "";
    messageStatus.textContent = "";
    latestMessageNotice = "";
    void loadContacts();
  } catch (error) {
    if (authenticated && requestGeneration === authGeneration) {
      messageStatus.textContent = error.message || "Unable to send message. Try again.";
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

  socket.addEventListener("close", () => {
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
      void loadFeed();
    }
  } catch {
    setAuthenticated(false);
    showView("login");
    showAuthStatus("Unable to check your sign-in status. Please try again.", "error");
  }
}

setAuthenticated(false);
showView("login");
restoreSession();
