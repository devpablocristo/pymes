import { expect, test } from "vitest";
import { calendarDate } from "./calendarDate";

test("returns the Argentine calendar day instead of the next UTC day", () => {
  expect(calendarDate(new Date("2026-07-25T02:30:00.000Z"))).toBe("2026-07-24");
});

test("accepts an explicit time zone for future country adapters", () => {
  expect(
    calendarDate(new Date("2026-07-25T02:30:00.000Z"), "Europe/Madrid"),
  ).toBe("2026-07-25");
});
