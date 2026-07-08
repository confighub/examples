# Build stage
FROM node:20-alpine AS builder
WORKDIR /app

# Build-time config baked into the static bundle by Vite. The OAuth client id is
# a public PKCE client (not a secret); CI passes it as a build arg. Both must be
# set for login to work — an empty client id renders the app's setup hint.
ARG VITE_CONFIGHUB_BASE_URL=https://hub.confighub.com
ARG VITE_OAUTH_CLIENT_ID
ENV VITE_CONFIGHUB_BASE_URL=$VITE_CONFIGHUB_BASE_URL
ENV VITE_OAUTH_CLIENT_ID=$VITE_OAUTH_CLIENT_ID

COPY app/package*.json ./
RUN npm ci
COPY app/ .
RUN npm run build

# Runtime stage
FROM nginx:alpine

# Create a dedicated non-root user with UID 1000
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

# Create and set up directories with proper permissions
RUN mkdir -p /var/cache/nginx /var/log/nginx /tmp /usr/share/nginx/html && \
    chown -R appuser:appgroup /var/cache/nginx /var/log/nginx /tmp /usr/share/nginx/html /etc/nginx && \
    chmod -R 755 /var/cache/nginx /var/log/nginx

# Copy built files and config. Static-only: no proxy, so no entrypoint templating.
COPY --from=builder /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/nginx.conf
RUN chown -R appuser:appgroup /usr/share/nginx/html /etc/nginx

# Switch to non-root user
USER 1000

EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
