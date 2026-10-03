import { test, expect } from "./fixtures";
import { login } from "./helpers/login";

test("generates an API key and uses it to upload and list files", async ({
  page,
  request,
}) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Settings" }).click();
  await expect(page).toHaveURL("/settings");

  await expect(page.getByText("No API key exists yet.")).toBeVisible();
  await page.getByRole("button", { name: "Generate API key" }).click();

  await expect(page.getByText("Copy your new API key now.")).toBeVisible();
  const apiKey = String(await page.locator("#api-key-value").textContent());
  expect(apiKey).toMatch(/^ps_[A-Za-z0-9]{40}$/);
  await expect(page.getByText("API key created on")).toBeVisible();

  {
    const response = await request.post("/api/v1/files", {
      headers: { Authorization: `Bearer ${apiKey}` },
      multipart: {
        file: {
          name: "api-upload.txt",
          mimeType: "text/plain",
          buffer: Buffer.from("uploaded through the API"),
        },
      },
    });
    expect(response.status()).toBe(200);
  }

  {
    const response = await request.get("/api/v1/files", {
      headers: { Authorization: `Bearer ${apiKey}` },
    });
    expect(response.status()).toBe(200);
    const files = await response.json();
    expect(files).toHaveLength(1);
    expect(files[0].filename).toBe("api-upload.txt");
  }

  await page.getByRole("menuitem", { name: "Files" }).click();
  await expect(page).toHaveURL("/files");
  await expect(
    page.getByRole("link", { name: "api-upload.txt" }),
  ).toBeVisible();
});

test("regenerating the API key invalidates the previous key", async ({
  page,
  request,
}) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Settings" }).click();
  await expect(page).toHaveURL("/settings");

  await page.getByRole("button", { name: "Generate API key" }).click();
  await expect(page.getByText("Copy your new API key now.")).toBeVisible();
  const oldApiKey = String(await page.locator("#api-key-value").textContent());

  await page.getByRole("button", { name: "Regenerate API key" }).click();
  const dialog = page.getByRole("dialog", { name: "Regenerate API key?" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Regenerate" }).click();
  await expect(dialog).toBeHidden();

  await expect(page.locator("#api-key-value")).not.toHaveText(oldApiKey);
  const newApiKey = String(await page.locator("#api-key-value").textContent());

  {
    const response = await request.get("/api/v1/files", {
      headers: { Authorization: `Bearer ${oldApiKey}` },
    });
    expect(response.status()).toBe(401);
  }

  {
    const response = await request.get("/api/v1/files", {
      headers: { Authorization: `Bearer ${newApiKey}` },
    });
    expect(response.status()).toBe(200);
  }
});

test("shows instructions for using the API", async ({ page }) => {
  await login(page);

  await page.getByRole("menuitem", { name: "System" }).hover();
  await page.getByRole("menuitem", { name: "Settings" }).click();
  await expect(page).toHaveURL("/settings");

  await page.getByRole("button", { name: "How to use the API" }).click();
  const dialog = page.getByRole("dialog", { name: "How to use the API" });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText("Authorization: Bearer YOUR_API_KEY", { exact: true }),
  ).toBeVisible();

  // Bootstrap focuses the dialog once its opening transition ends, and only
  // then does it handle the Escape key.
  await expect(dialog).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(dialog).toBeHidden();
});
