import { expect, test } from "@playwright/test";

test("reserva un turno público sin autenticarse", async ({ page }) => {
  await page.goto("/reservar/centro-norte");
  await expect(page.getByRole("heading", { name: "Centro Norte" })).toBeVisible();
  if (process.env.PYMES_VISUAL_CAPTURE) {
    await page.screenshot({ path: `/tmp/pymes-public-booking-${test.info().project.name}.png`, fullPage: true });
  }
  await page.getByText("Consulta inicial", { exact: true }).click();
  await page.getByRole("button", { name: "Ver horarios" }).click();
  await expect(page.getByRole("heading", { name: "Elegí el horario que te conviene" })).toBeVisible();

  const firstSlot = page.locator(".slot-grid input[type=radio]").first();
  await expect(firstSlot).toBeAttached();
  await firstSlot.check();
  await page.getByRole("button", { name: "Continuar" }).click();
  await page.getByLabel("Nombre y apellido").fill("Lucía Público");
  await page.getByLabel("Email").fill("lucia@example.invalid");
  await page.getByLabel("WhatsApp").fill("+5491199999999");
  await page.getByRole("button", { name: "Confirmar reserva" }).click();
  await expect(page.getByRole("heading", { name: "Tu horario quedó reservado" })).toBeVisible();
});

test("consume una acción pública opaca", async ({ page }) => {
  const token = "a".repeat(90);
  await page.goto(`/agenda/accion/${token}?purpose=confirm&version=1`);
  await expect(page.getByRole("heading", { name: "Confirmar turno" })).toBeVisible();
  await page.getByRole("button", { name: "Confirmar", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Listo, guardamos el cambio" })).toBeVisible();
});
