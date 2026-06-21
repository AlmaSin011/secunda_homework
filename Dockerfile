FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Статическая сборка (CGO=0) — бинарь работает в scratch/distroless.
# -trimpath убирает локальные пути из бинаря (воспроизводимая сборка).
# -ldflags="-s -w" режет отладочную информацию → меньше размер.
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/app ./cmd/app

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/migrate ./cmd/migrate


FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=builder /out/app /app/app
COPY --from=builder /out/migrate /app/migrate

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/app/app"]
