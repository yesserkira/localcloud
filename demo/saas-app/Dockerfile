# syntax=docker/dockerfile:1
FROM node:20-alpine AS base
WORKDIR /app
COPY package.json package-lock.json* ./
RUN npm install --production
COPY src ./src
COPY migrations ./migrations

FROM base AS api
EXPOSE 3000
CMD ["node", "src/api/server.js"]

FROM base AS worker
CMD ["node", "src/worker/index.js"]
