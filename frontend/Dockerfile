FROM node:24-alpine AS builder
# `npm ci` runs the project's prepare script (vp config), which needs git.
RUN apk add --no-cache git=2.54.0-r0
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM alpine:3.19
WORKDIR /app
# Output the build where the shared volume (frontend-dist) expects it.
COPY --from=builder /app/dist /app/dist
# This container only publishes built static assets to the shared volume
# consumed by the nginx reverse proxy; it does not serve traffic itself.
CMD ["sleep", "infinity"]
