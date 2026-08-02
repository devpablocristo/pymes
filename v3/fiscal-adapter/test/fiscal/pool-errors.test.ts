import assert from "node:assert/strict";
import { EventEmitter } from "node:events";
import test from "node:test";
import {
  observePoolErrors,
  type DatabasePoolEvent,
} from "../../src/fiscal/repository/helpers/pool-errors.js";

test("idle PostgreSQL errors are observed without becoming unhandled process errors", () => {
  const source = new EventEmitter();
  const events: DatabasePoolEvent[] = [];
  observePoolErrors(source, (event) => events.push(event));
  const connectionError = new Error("connection contained sensitive details");
  Object.assign(connectionError, { code: "57P01" });

  source.emit("error", connectionError);
  source.emit("error", Object.assign(new Error("bad"), { code: "unsafe code" }));

  assert.deepEqual(events, [
    { type: "fiscal_database_pool_error", code: "57P01" },
    {
      type: "fiscal_database_pool_error",
      code: "DATABASE_CONNECTION_LOST",
    },
  ]);
  assert.equal(JSON.stringify(events).includes("sensitive"), false);
});
