# ── build stage ───────────────────────────────────────────────
FROM golang:1.27 AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# modernc.org/sqlite is pure Go, so CGO is not required.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w" -o /out/api ./cmd/api

# ── runtime stage ─────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/api /api

ENV PORT=8080 \
    DB_PATH=/data/expense_tracker.db

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/api"]
