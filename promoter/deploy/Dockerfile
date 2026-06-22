# Build stage
FROM node:20-alpine AS builder
WORKDIR /app
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

# Copy built files and config
COPY --from=builder /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/nginx.conf.template
COPY deploy/docker-entrypoint.sh /docker-entrypoint.sh
RUN chown -R appuser:appgroup /usr/share/nginx/html /etc/nginx && \
    chmod +x /docker-entrypoint.sh

# Switch to non-root user
USER 1000

EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
