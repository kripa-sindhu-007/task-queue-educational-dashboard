FROM node:20-alpine AS builder

WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .

ENV NEXT_PUBLIC_API_URL=http://localhost:8080
RUN npm run build

FROM node:20-alpine
WORKDIR /app

LABEL org.opencontainers.image.source="https://github.com/kripa-sindhu-007/weekend_project_1"
LABEL org.opencontainers.image.description="Task Queue Frontend — Next.js real-time dashboard"
LABEL org.opencontainers.image.licenses="MIT"

COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

ENV PORT=3000
ENV HOSTNAME=0.0.0.0
EXPOSE 3000
CMD ["node", "server.js"]
