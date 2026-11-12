import { expect, test } from "@playwright/test";
const username = process.env.E2E_USERNAME ?? "admin";
const password = process.env.E2E_PASSWORD ?? "password";

test("the login form rejects a wrong password and keeps the session across a reload", async ({ page }) => {
  await page.goto("/");
  await page.getByPlaceholder("Username").fill(username);
  await page.getByPlaceholder("Password").fill(`${password}-wrong`);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert")).toContainText("Incorrect");
  await page.getByPlaceholder("Password").fill(password);
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("button", { name: "Tools" })).toBeVisible();
  await page.reload();
  await expect(page.getByRole("button", { name: "Tools" })).toBeVisible();
});
