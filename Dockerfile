# syntax=docker/dockerfile:1

# ---- frontend build stage ----
# Build the Vite + React SPA. Its output (dist) is copied into the runtime image
# and served by the Go backend from MAILFOLD_FRONTEND_DIR.
# Base images are pulled from the AWS ECR Public mirror of Docker Hub's official
# images to avoid Docker Hub's anonymous pull rate limits in CI and deploys.
FROM public.ecr.aws/docker/library/node:20-alpine AS frontend
WORKDIR /web
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ---- backend build stage ----
# Compile a static Linux binary.
FROM public.ecr.aws/docker/library/golang:1.25-alpine AS build
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

# The backend serves the built SPA from this directory.
COPY --from=frontend /web/dist /app/frontend/dist
ENV MAILFOLD_FRONTEND_DIR=/app/frontend/dist

EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/mailfold"]
