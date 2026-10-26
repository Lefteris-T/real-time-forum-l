const panels = document.querySelectorAll("[data-view-panel]");
const registrationForm = document.querySelector("#registration-form");
const registrationResult = document.querySelector("#registration-result");
const loginForm = document.querySelector("#login-form");
const loginResult = document.querySelector("#login-result");
const postForm = document.querySelector("#post-form");
const postResult = document.querySelector("#post-result");
const postCategories = document.querySelector("#post-categories");
const postSubmitButton = postForm.querySelector('button[type="submit"]');
const logoutButton = document.querySelector("[data-logout]");
const authStatus = document.querySelector("#auth-status");
const guestNavigation = document.querySelectorAll("[data-guest-only]");
const authenticatedNavigation = document.querySelectorAll("[data-auth-required]");

let authenticated = false;
let categoriesLoaded = false;
let categoriesLoading = false;

function showView(viewName) {
  for (const panel of panels) {
    panel.hidden = panel.dataset.viewPanel !== viewName;
  }
}

function setAuthenticated(value) {
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
  const button = event.target.closest("[data-view]");

  if (!button) {
    return;
  }

  showView(button.dataset.view);

  if (button.dataset.view === "create-post") {
    void loadCategories();
  }
});

async function loadCategories() {
  if (!authenticated || categoriesLoaded || categoriesLoading) {
    return;
  }

  categoriesLoading = true;
  postCategories.textContent = "Loading categories…";
  postSubmitButton.disabled = true;

  try {
    const response = await fetch("/api/categories", {
      method: "GET",
      credentials: "same-origin",
      headers: { Accept: "application/json" },
    });
    const result = await readJSON(response);

    if (!authenticated) {
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
    postCategories.textContent = "Unable to load categories. Select Create Post to try again.";
    postResult.textContent = "Unable to load categories. Please try again.";
    postResult.dataset.status = "error";
    postResult.hidden = false;
  } finally {
    categoriesLoading = false;
  }
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

    postResult.textContent = response.ok
      ? `${result.message || "Post created."} ID: ${result.post_id}.`
      : result.error || "Unable to create post. Please check the title, body, and categories.";
    postResult.dataset.status = response.ok ? "success" : "error";
    postResult.hidden = false;

    if (response.ok) {
      postForm.reset();
    }
  } catch {
    postResult.textContent = "Unable to create post. Please try again.";
    postResult.dataset.status = "error";
    postResult.hidden = false;
  } finally {
    postSubmitButton.disabled = !categoriesLoaded;
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
  } catch {
    setAuthenticated(false);
    showView("login");
    showAuthStatus("Unable to check your sign-in status. Please try again.", "error");
  }
}

setAuthenticated(false);
showView("login");
restoreSession();
