FROM golang:1.26.5@sha256:3aff6657219a4d9c14e27fb1d8976c49c29fddb70ba835014f477e1c70636647 AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate && \
    CGO_ENABLED=0 go build -o /out/provision-org ./cmd/provision-org && \
    CGO_ENABLED=0 go build -o /out/fake-service ./cmd/fake-service && \
    CGO_ENABLED=0 go build -o /out/pergo-fake ./cmd/pergo-fake && \
    CGO_ENABLED=0 go build -o /out/healthcheck ./cmd/healthcheck

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS web-deps
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts

FROM web-deps AS web-build
ARG VITE_CLERK_PUBLISHABLE_KEY
ARG VITE_PYMES_ORGANIZATION_SLUG
ENV VITE_CLERK_PUBLISHABLE_KEY=$VITE_CLERK_PUBLISHABLE_KEY
ENV VITE_PYMES_ORGANIZATION_SLUG=$VITE_PYMES_ORGANIZATION_SLUG
COPY web/ ./
RUN npm run build

FROM web-deps AS web-build-local
COPY web/ ./
RUN npm run build -- --mode e2e

FROM nginxinc/nginx-unprivileged:1.27-alpine@sha256:65e3e85dbaed8ba248841d9d58a899b6197106c23cb0ff1a132b7bfe0547e4c0 AS web-runtime
ENV NGINX_ENVSUBST_FILTER="^(PYMES_API_UPSTREAM|PYMES_RELEASE_MARKER|PYMES_PREFLIGHT_TAG|PYMES_PREFLIGHT_TOKEN)$"
COPY web/nginx.conf /etc/nginx/templates/default.conf.template

FROM web-runtime AS web
COPY --from=web-build /src/web/dist /usr/share/nginx/html

FROM web-runtime AS web-local
COPY --from=web-build-local /src/web/dist /usr/share/nginx/html

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS api
COPY --from=build /out/api /api
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS worker
COPY --from=build /out/worker /worker
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/worker"]

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS migrate
COPY --from=build /out/migrate /migrate
COPY db/migrations /migrations
ENV PYMES_MIGRATIONS_DIR=/migrations
ENTRYPOINT ["/migrate"]

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS provision-org
COPY --from=build /out/provision-org /provision-org
ENTRYPOINT ["/provision-org"]

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS fake-service
COPY --from=build /out/fake-service /fake-service
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/fake-service"]

FROM gcr.io/distroless/static-debian12:nonroot@sha256:f5b485ea962d9bd1186b2f6b3a061191539b905b82ec395de78cbfae51f20e35 AS pergo-fake
COPY --from=build /out/pergo-fake /pergo-fake
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/pergo-fake"]

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS fiscal-deps
WORKDIR /src/fiscal-adapter
COPY fiscal-adapter/package.json fiscal-adapter/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS fiscal-build
WORKDIR /src/fiscal-adapter
COPY fiscal-adapter/package.json fiscal-adapter/package-lock.json ./
RUN npm ci --ignore-scripts
COPY fiscal-adapter/ ./
RUN npm run build

FROM node:22-alpine@sha256:c610fcdfb1d5b4740dd70c284ed3cb16bb857e0f7166196e36a5501df7a3aa32 AS fiscal-adapter
ENV NODE_ENV=production
USER node
WORKDIR /app
COPY --from=fiscal-build --chown=node:node /src/fiscal-adapter/dist ./dist
COPY --from=fiscal-deps --chown=node:node /src/fiscal-adapter/node_modules ./node_modules
COPY --from=fiscal-deps --chown=node:node /src/fiscal-adapter/package.json ./package.json
ENTRYPOINT ["node", "dist/src/cmd/api.js"]

FROM postgres:16-alpine@sha256:57c72fd2a128e416c7fcc499958864df5301e940bca0a56f58fddf30ffc07777 AS fiscal-migrate
COPY fiscal-adapter/migrations /migrations
COPY fiscal-adapter/scripts/migrate.sh /migrate.sh
RUN chmod 0555 /migrate.sh
USER postgres
ENTRYPOINT ["/migrate.sh"]
