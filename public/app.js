// Fill the event textarea from the clipboard (needs HTTPS or localhost).
const paste = document.querySelector("[data-paste]");
const field = document.querySelector("#json");
if (paste && field && navigator.clipboard && navigator.clipboard.readText) {
  paste.hidden = false;
  paste.addEventListener("click", async () => {
    try {
      field.value = await navigator.clipboard.readText();
      field.focus();
    } catch {
      paste.textContent = "Pristup odbijen — zalijepi ručno";
    }
  });
}

// Ask before submitting destructive forms (e.g. deleting an event).
document.addEventListener("submit", (event) => {
  const message = event.target.dataset && event.target.dataset.confirm;
  if (message && !window.confirm(message)) event.preventDefault();
});
