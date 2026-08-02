import { describe, expect, it } from "vitest";
import { ClientCommandIdentity } from "./clientCommandIdentity";

describe("ClientCommandIdentity", () => {
  it("reutiliza el ID sólo mientras el snapshot del comando sea idéntico", () => {
    const identity = new ClientCommandIdentity();
    const first = identity.forPayload({ slot: "2026-08-03T12:00:00Z", people: 1 });

    expect(
      identity.forPayload({ slot: "2026-08-03T12:00:00Z", people: 1 }),
    ).toBe(first);
    expect(
      identity.forPayload({ slot: "2026-08-03T12:00:00Z", people: 2 }),
    ).not.toBe(first);
  });

  it("genera otro ID después de completar el comando", () => {
    const identity = new ClientCommandIdentity();
    const first = identity.forPayload({ value: "same" });
    identity.reset();

    expect(identity.forPayload({ value: "same" })).not.toBe(first);
  });
});
