FROM node:22-alpine AS build
WORKDIR /build
COPY apps/web/package*.json apps/web/
COPY apps/ops/package*.json apps/ops/
RUN cd apps/web && npm ci
RUN cd apps/ops && npm ci
COPY apps/web/ apps/web/
COPY apps/ops/ apps/ops/
ENV VITE_API_BASE_URL=/api/v1
ENV VITE_OPS_URL=/staff/
ENV VITE_PUBLIC_URL=/
RUN cd apps/web && npm run build
RUN cd apps/ops && npm run build -- --base=/staff/
FROM caddy:2-alpine
COPY --from=build /build/apps/web/dist /srv/public
COPY --from=build /build/apps/ops/dist /srv/staff
COPY infra/Caddyfile /etc/caddy/Caddyfile
