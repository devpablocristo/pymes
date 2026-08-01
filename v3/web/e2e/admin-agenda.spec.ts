import { expect, test } from "@playwright/test";
import { DateTime } from "luxon";

test("opera agenda, disponibilidad y cola con fake contractual", async ({ page }) => {
  await page.goto("/app/agenda");
  await expect(page.getByRole("heading", { name: "Agenda", exact: true })).toBeVisible();
  await expect(page.getByText("Centro Norte")).toBeVisible();
  await expect
    .poll(() =>
      page.evaluate(
        () =>
          document.documentElement.scrollWidth <=
          document.documentElement.clientWidth + 1,
      ),
    )
    .toBe(true);

  await page.getByRole("button", { name: "+ Nuevo turno" }).click();
  const dialog = page.getByRole("dialog", { name: "Nuevo turno" });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel("Servicio").selectOption({ index: 1 });
  await dialog.getByLabel("Comienza").fill(
    DateTime.now()
      .setZone("America/Argentina/Buenos_Aires")
      .set({ hour: 14, minute: 0, second: 0, millisecond: 0 })
      .toFormat("yyyy-MM-dd'T'HH:mm"),
  );
  await dialog.getByLabel("Cliente").fill("Cliente E2E");
  await dialog.getByLabel("Email").fill("cliente@example.invalid");
  await dialog.getByLabel("WhatsApp").fill("+5491100000000");
  await dialog.getByRole("checkbox").nth(1).check();
  await dialog.getByRole("button", { name: "Crear turno" }).click();
  await expect(page.getByText("Turno creado.")).toBeVisible();

  await page.locator(".booking-event").first().click();
  const details = page.getByRole("complementary", { name: "Detalle del turno" });
  await expect(details).toBeVisible();
  await page.getByRole("button", { name: "Editar" }).click();
  const editDialog = page.getByRole("dialog", { name: "Editar turno" });
  await expect(editDialog.getByLabel(/^Cliente/)).toHaveAttribute("readonly", "");
  await editDialog.getByLabel("Nota interna").fill("Seguimiento desde E2E");
  await editDialog.getByRole("button", { name: "Guardar cambios" }).click();
  await expect(page.getByText("Turno actualizado.")).toBeVisible();
  await details.getByRole("button", { name: "Cerrar detalle" }).click();

  await page.getByRole("button", { name: "Día" }).click();
  await page.getByRole("button", { name: "Mes" }).click();
  await page.getByRole("button", { name: "Lista", exact: true }).click();
  await page.getByRole("button", { name: "Semana" }).click();

  await page.getByRole("button", { name: "Disponibilidad" }).click();
  await expect(page.getByRole("heading", { name: "Disponibilidad habitual" })).toBeVisible();
  await page.getByRole("button", { name: "Bloquear horario" }).click();
  const blockDialog = page.getByRole("dialog", { name: "Bloquear disponibilidad" });
  await blockDialog.getByLabel("Detalle visible para el equipo").fill("Capacitación interna");
  await blockDialog.getByRole("button", { name: "Crear bloqueo" }).click();
  await expect(page.getByText("Bloqueo creado.")).toBeVisible();
  await expect(page.getByText("Capacitación interna")).toBeVisible();

  await page.getByRole("button", { name: "Cola" }).click();
  await expect(page.getByRole("heading", { name: "Cola de hoy" })).toBeVisible();
  await page.getByRole("button", { name: "Llamar" }).click();
  await expect(page.getByRole("button", { name: "Atender" })).toBeVisible();
});

test("los filtros operativos permanecen accesibles por nombre", async ({ page }) => {
  await page.goto("/app/agenda");
  const filters = page.getByRole("region", { name: "Filtros de agenda" });
  await expect(filters.getByLabel("Sucursal")).toBeVisible();
  await expect(filters.getByLabel("Servicio")).toBeVisible();
  await expect(filters.getByLabel("Profesional o recurso")).toBeVisible();
  await expect(page.locator("main#main-content")).toHaveCount(1);
});
