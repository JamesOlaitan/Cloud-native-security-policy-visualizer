FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY ui/package.json ui/package*.json* ui/yarn.lock* ./

# Install dependencies
RUN npm install || yarn install

# Copy source
COPY ui/ .

# Build
RUN npm run build || yarn build

# Final stage - nginx
FROM nginx:alpine

# Copy built files
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx config
RUN echo 'server { \
    listen 3000; \
    location / { \
        root /usr/share/nginx/html; \
        index index.html; \
        try_files $uri $uri/ /index.html; \
    } \
}' > /etc/nginx/conf.d/default.conf

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]

