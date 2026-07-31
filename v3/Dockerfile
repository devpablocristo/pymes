FROM golang:1.26.5 AS build
WORKDIR /src/backend
COPY backend/go.mod ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -o /out/worker ./cmd/worker && \
    CGO_ENABLED=0 go build -o /out/provision-org ./cmd/provision-org && \
    CGO_ENABLED=0 go build -o /out/fake-service ./cmd/fake-service && \
    CGO_ENABLED=0 go build -o /out/healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12 AS api
COPY --from=build /out/api /api
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/api"]

FROM gcr.io/distroless/static-debian12 AS worker
COPY --from=build /out/worker /worker
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/worker"]

FROM gcr.io/distroless/static-debian12 AS provision-org
COPY --from=build /out/provision-org /provision-org
ENTRYPOINT ["/provision-org"]

FROM gcr.io/distroless/static-debian12 AS fake-service
COPY --from=build /out/fake-service /fake-service
COPY --from=build /out/healthcheck /healthcheck
ENTRYPOINT ["/fake-service"]

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
