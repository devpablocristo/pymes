FROM golang:1.26.5 AS build
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

FROM node:22-alpine AS web-deps
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --ignore-scripts

FROM web-deps AS web-build
ARG VITE_API_BASE_URL
ARG VITE_CLERK_PUBLISHABLE_KEY
ARG VITE_PYMES_ORGANIZATION_SLUG
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_CLERK_PUBLISHABLE_KEY=$VITE_CLERK_PUBLISHABLE_KEY
ENV VITE_PYMES_ORGANIZATION_SLUG=$VITE_PYMES_ORGANIZATION_SLUG
COPY web/ ./
RUN npm run build

FROM web-deps AS web-build-local
COPY web/ ./
RUN npm run build -- --mode e2e

FROM nginxinc/nginx-unprivileged:1.27-alpine AS web-runtime
COPY web/nginx.conf /etc/nginx/conf.d/default.conf

FROM web-runtime AS web
COPY --from=web-build /src/web/dist /usr/share/nginx/html

FROM web-runtime AS web-local
COPY --from=web-build-local /src/web/dist /usr/share/nginx/html

FROM gcr.io/distroless/static-debian12:nonroot AS api
COPY --from=build /out/api /api
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=build /out/worker /worker
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/worker"]

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=build /out/migrate /migrate
COPY db/migrations /migrations
ENV PYMES_MIGRATIONS_DIR=/migrations
ENTRYPOINT ["/migrate"]

FROM gcr.io/distroless/static-debian12:nonroot AS provision-org
COPY --from=build /out/provision-org /provision-org
ENTRYPOINT ["/provision-org"]

FROM gcr.io/distroless/static-debian12:nonroot AS fake-service
COPY --from=build /out/fake-service /fake-service
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/fake-service"]

FROM gcr.io/distroless/static-debian12:nonroot AS pergo-fake
COPY --from=build /out/pergo-fake /pergo-fake
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/pergo-fake"]

FROM node:22-alpine AS fiscal-deps
WORKDIR /src/fiscal-adapter
COPY fiscal-adapter/package.json fiscal-adapter/package-lock.json ./
RUN npm ci --omit=dev --ignore-scripts

FROM node:22-alpine AS fiscal-build
WORKDIR /src/fiscal-adapter
COPY fiscal-adapter/package.json fiscal-adapter/package-lock.json ./
RUN npm ci --ignore-scripts
COPY fiscal-adapter/ ./
RUN npm run build

FROM node:22-alpine AS fiscal-adapter
ENV NODE_ENV=production
USER node
WORKDIR /app
COPY --from=fiscal-build --chown=node:node /src/fiscal-adapter/dist ./dist
COPY --from=fiscal-deps --chown=node:node /src/fiscal-adapter/node_modules ./node_modules
COPY --from=fiscal-deps --chown=node:node /src/fiscal-adapter/package.json ./package.json
ENTRYPOINT ["node", "dist/src/cmd/api.js"]

FROM postgres:16-alpine AS fiscal-migrate
COPY fiscal-adapter/migrations /migrations
USER postgres
ENTRYPOINT ["sh", "-ec", "for file in /migrations/*.sql; do psql \"$FISCAL_DATABASE_URL\" -v ON_ERROR_STOP=1 -f \"$file\"; done"]
