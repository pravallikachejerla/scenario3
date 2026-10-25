# Builds the api, migrate, seed and worker binaries.
#
# The default build tag set uses only the Go standard library (see
# cmd/api/main.go). Pass BUILD_TAGS=postgres to also compile the
# PostgreSQL-backed storage path and driver, once go.mod/go.sum has
# been updated with `go mod tidy -tags postgres` in an environment
# with package-registry network access (the SOMA Genesis Sandbox).
FROM golang:1.22-alpine AS build
ARG BUILD_TAGS=""
WORKDIR /src
COPY go.mod ./
COPY . .
RUN go build -tags "${BUILD_TAGS}" -o /out/api ./cmd/api \
 && go build -tags "${BUILD_TAGS}" -o /out/migrate ./cmd/migrate \
 && go build -tags "${BUILD_TAGS}" -o /out/seed ./cmd/seed \
 && go build -tags "${BUILD_TAGS}" -o /out/worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates wget
COPY --from=build /out/api /app/api
COPY --from=build /out/migrate /app/migrate
COPY --from=build /out/seed /app/seed
COPY --from=build /out/worker /app/worker
COPY migrations /app/migrations
EXPOSE 8081
ENTRYPOINT ["/app/api"]