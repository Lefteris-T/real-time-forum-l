const panels = document.querySelectorAll("[data-view-panel]");
const registrationForm = document.querySelector("#registration-form");
const registrationResult = document.querySelector("#registration-result");

function showView(viewName) {
  for (const panel of panels) {
    panel.hidden = panel.dataset.viewPanel !== viewName;
  }
}

document.addEventListener("click", (event) => {
  const button = event.target.closest("[data-view]");

  if (!button) {
    return;
  }

  showView(button.dataset.view);
});

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

    let result;
    try {
      result = await response.json();
    } catch {
      throw new Error("The server returned an unreadable response.");
    }

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

showView("login");
