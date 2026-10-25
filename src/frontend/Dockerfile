# syntax=docker/dockerfile:1
# Build context is the repository root so the SPA can bundle wiki markdown
# from the calculation-* folders.
FROM node:22-bookworm-slim AS deps
WORKDIR /repo/src/frontend
COPY src/frontend/package.json src/frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci

FROM deps AS dev
COPY src/frontend ./
COPY src/calculation-technical-indicator /repo/src/calculation-technical-indicator
COPY src/calculation-signal /repo/src/calculation-signal
EXPOSE 3000
CMD ["npm", "run", "dev", "--", "--host", "0.0.0.0", "--port", "3000"]

FROM deps AS build
COPY src/frontend ./
COPY src/calculation-technical-indicator /repo/src/calculation-technical-indicator
COPY src/calculation-signal /repo/src/calculation-signal
ARG VITE_EXPERIMENT_API_URL=
ENV VITE_EXPERIMENT_API_URL=$VITE_EXPERIMENT_API_URL
RUN npm run build

FROM deps AS prod
COPY --from=build /repo/src/frontend/dist ./dist
COPY src/frontend/vite.config.ts ./
EXPOSE 3000
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "3000"]
