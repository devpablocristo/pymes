FROM golang:1.26.5-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN GOWORK=off go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOWORK=off go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/pymes-v2-api \
    ./cmd/api

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=build /out/pymes-v2-api /usr/local/bin/pymes-v2-api

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/pymes-v2-api"]
