import createClient from "openapi-fetch";
import type { paths } from "./generated";
import type {
  AvailabilityBlockInput,
  AvailabilityQuery,
  BookingAction,
  BookingInput,
  PublicActionInput,
  QueueAdvanceInput,
  QueueTicketInput,
  WaitlistInput,
} from "../domain/scheduling";
import type { RequestIdentity, SchedulingGateway } from "./SchedulingGateway";
import { newIdempotencyKey } from "./SchedulingGateway";
import { SchedulingApiError } from "./errors";

type ApiResult<T> = {
  data?: T;
  error?: unknown;
  response: Response;
};

function parseError(error: unknown, response: Response): SchedulingApiError {
  if (typeof error === "object" && error !== null && "code" in error) {
    const code = String(error.code);
    const message = "message" in error ? String(error.message) : code;
    return new SchedulingApiError(code as ConstructorParameters<typeof SchedulingApiError>[0], message, response.status);
  }
  if (response.status === 403) {
    return new SchedulingApiError("FORBIDDEN", "Acceso denegado", response.status);
  }
  return new SchedulingApiError("UNKNOWN", `La API respondió ${response.status}`, response.status);
}

function unwrap<T>(result: ApiResult<T>): T {
  if (result.data !== undefined) {
    return result.data;
  }
  throw parseError(result.error, result.response);
}

async function authenticatedHeaders(identity: RequestIdentity): Promise<Headers> {
  const headers = new Headers();
  const token = await identity.getToken();
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return headers;
}

type MutationContext = {
  headers: Headers;
  params: { "Idempotency-Key": string; "X-Source-Version": number };
};

async function authenticatedMutation(identity: RequestIdentity, mutation: string): Promise<MutationContext> {
  const headers = await authenticatedHeaders(identity);
  const key = newIdempotencyKey(mutation);
  headers.set("Idempotency-Key", key);
  headers.set("X-Source-Version", "1");
  return { headers, params: { "Idempotency-Key": key, "X-Source-Version": 1 } };
}

function publicMutation(mutation: string): MutationContext {
  const headers = new Headers();
  const key = newIdempotencyKey(mutation);
  headers.set("Idempotency-Key", key);
  headers.set("X-Source-Version", "1");
  return { headers, params: { "Idempotency-Key": key, "X-Source-Version": 1 } };
}

export function createHttpSchedulingGateway(baseUrl: string): SchedulingGateway {
  const client = createClient<paths>({ baseUrl });

  return {
    async listBranches(identity) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/branches", {
          params: { path: { organizationId: identity.organizationId } },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async listServices(identity) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/services", {
          params: { path: { organizationId: identity.organizationId } },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async listResources(identity, branchId) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/resources", {
          params: {
            path: { organizationId: identity.organizationId },
            query: branchId ? { branch_id: branchId } : {},
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async listBookings(identity, branchId, range) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/bookings", {
          params: {
            path: { organizationId: identity.organizationId },
            query: { branch_id: branchId, from: range.from, until: range.until },
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async createBooking(identity, input) {
      const mutation = await authenticatedMutation(identity, "booking:create");
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/bookings", {
          params: { path: { organizationId: identity.organizationId }, header: mutation.params },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async rescheduleBooking(identity, bookingId, expectedVersion, startAt, durationMinutes, allocations) {
      const mutation = await authenticatedMutation(identity, `booking:${bookingId}:reschedule`);
      const body = allocations
        ? { expected_version: expectedVersion, start_at: startAt, duration_minutes: durationMinutes, allocations }
        : { expected_version: expectedVersion, start_at: startAt, duration_minutes: durationMinutes };
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/bookings/{bookingId}/reschedule", {
          params: { path: { organizationId: identity.organizationId, bookingId }, header: mutation.params },
          headers: mutation.headers,
          body,
        }),
      );
    },
    async transitionBooking(identity, bookingId, action: BookingAction, expectedVersion, reason) {
      const mutation = await authenticatedMutation(identity, `booking:${bookingId}:${action}`);
      const body = reason ? { expected_version: expectedVersion, reason } : { expected_version: expectedVersion };
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/bookings/{bookingId}/{action}", {
          params: {
            path: { organizationId: identity.organizationId, bookingId, action },
            header: mutation.params,
          },
          headers: mutation.headers,
          body,
        }),
      );
    },
    async calculateAvailability(identity, query) {
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/availability", {
          params: { path: { organizationId: identity.organizationId } },
          headers: await authenticatedHeaders(identity),
          body: query,
        }),
      );
    },
    async listAvailabilityRules(identity, branchId) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/availability/rules", {
          params: {
            path: { organizationId: identity.organizationId },
            query: { branch_id: branchId },
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async listBlocks(identity, branchId, range) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/blocks", {
          params: {
            path: { organizationId: identity.organizationId },
            query: { branch_id: branchId, from: range.from, until: range.until },
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async createBlock(identity, input: AvailabilityBlockInput) {
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/blocks", {
          params: { path: { organizationId: identity.organizationId } },
          headers: await authenticatedHeaders(identity),
          body: input,
        }),
      );
    },
    async listWaitlist(identity, branchId) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/waitlist", {
          params: {
            path: { organizationId: identity.organizationId },
            query: { branch_id: branchId },
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async createWaitlistEntry(identity, input: WaitlistInput) {
      const mutation = await authenticatedMutation(identity, "waitlist:create");
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/waitlist", {
          params: { path: { organizationId: identity.organizationId }, header: mutation.params },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async listQueue(identity, branchId) {
      return unwrap(
        await client.GET("/api/v1/organizations/{organizationId}/scheduling/queue", {
          params: {
            path: { organizationId: identity.organizationId },
            query: { branch_id: branchId },
          },
          headers: await authenticatedHeaders(identity),
        }),
      );
    },
    async createQueueTicket(identity, input: QueueTicketInput) {
      const mutation = await authenticatedMutation(identity, "queue:create");
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/queue", {
          params: { path: { organizationId: identity.organizationId }, header: mutation.params },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async advanceQueueTicket(identity, ticketId, input: QueueAdvanceInput) {
      const mutation = await authenticatedMutation(identity, `queue:${ticketId}:${input.status}`);
      return unwrap(
        await client.POST("/api/v1/organizations/{organizationId}/scheduling/queue/{ticketId}", {
          params: {
            path: { organizationId: identity.organizationId, ticketId },
            header: mutation.params,
          },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async getPublicCatalog(organizationSlug) {
      return unwrap(
        await client.GET("/api/v1/public/scheduling/{organizationSlug}/catalog", {
          params: { path: { organizationSlug } },
        }),
      );
    },
    async calculatePublicAvailability(organizationSlug, query: AvailabilityQuery) {
      return unwrap(
        await client.POST("/api/v1/public/scheduling/{organizationSlug}/availability", {
          params: { path: { organizationSlug } },
          body: query,
        }),
      );
    },
    async createPublicBooking(organizationSlug, input: BookingInput) {
      const mutation = publicMutation("public-booking:create");
      return unwrap(
        await client.POST("/api/v1/public/scheduling/{organizationSlug}/bookings", {
          params: { path: { organizationSlug }, header: mutation.params },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async createPublicWaitlistEntry(organizationSlug, input: WaitlistInput) {
      const mutation = publicMutation("public-waitlist:create");
      return unwrap(
        await client.POST("/api/v1/public/scheduling/{organizationSlug}/waitlist", {
          params: { path: { organizationSlug }, header: mutation.params },
          headers: mutation.headers,
          body: input,
        }),
      );
    },
    async consumePublicAction(token, input: PublicActionInput) {
      const mutation = publicMutation(`public-action:${input.purpose}`);
      const result = await client.POST("/api/v1/public/scheduling/actions/{token}", {
        params: { path: { token }, header: mutation.params },
        headers: mutation.headers,
        body: input,
      });
      if (!result.response.ok) {
        throw parseError(result.error, result.response);
      }
    },
  };
}
