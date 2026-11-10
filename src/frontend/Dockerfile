# syntax=docker/dockerfile:1
# Dev server image (no task runs it; task frontend:dev runs on the host). Build context is the
# repository root so the SPA can bundle wiki markdown from the calculation-* folders. The shipped SPA is
# built by scripts/frontend-deploy.sh and uploaded to S3 (infra/terraform/frontend.tf).
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
