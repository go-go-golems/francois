// Shared plain notice for public actions, launch failures and hosted runtime errors.
export function serviceNotice(message: string, fullPage = false): HTMLElement {
  const box = document.createElement("section");
  box.className = fullPage ? "service-notice service-notice-page" : "service-notice";
  box.setAttribute("role", "status");
  const text = document.createElement("p");
  text.textContent = message;
  box.append(text);
  if (fullPage) {
    const back = document.createElement("a");
    back.href = "/";
    back.className = "u";
    back.textContent = "Back to the site";
    box.append(back);
  }
  return box;
}
export const storageMutation = (method: string) =>
  ["db.set", "db.update", "db.delete", "db.acquire", "assets.delete", "comments.create", "comments.reply", "comments.resolve", "comments.delete"].includes(method);
