# syntax=docker/dockerfile:1

# ---- build stage ----
# Compile a static Linux binary. The build context is the repository root so
# that a built frontend (frontend/dist) can be copied in as well when present.
FROM golang:1.25-alpine AS build
WORKDIR /src/backend

# Download modules first for better layer caching (rebuilds skip this unless
# go.mod/go.sum change).
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Build the server. CGO is disabled to produce a fully static binary suitable
# for the distroless runtime image below.
COPY backend/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/mailfold ./cmd/mailfold

# ---- runtime stage ----
# distroless/static keeps the image tiny and runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/mailfold /app/mailfold

# The backend serves the built SPA from this directory when it exists. Mount or
# COPY frontend/dist here once the frontend design is implemented.
ENV MAILFOLD_FRONTEND_DIR=/app/frontend/dist

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/mailfold"]
