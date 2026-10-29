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
const guestNavigation = document.querySelectorAll("[data-guest-only]");
const authenticatedNavigation = document.querySelectorAll("[data-auth-required]");

let authenticated = false;
let authGeneration = 0;
let categoriesLoaded = false;
let categoriesLoading = false;
let feedRequest = null;
let selectedPostID = null;

function showView(viewName) {
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
  } else {
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
    setStatus(detailStatus, "");
    setStatus(commentsStatus, "");
  }
}

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
