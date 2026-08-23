# Runtime image.
#
# Instead of pulling oven/bun from Docker Hub, it extends the internal
# miaozao/bun-base image (built from a bare Debian base + official Bun binary).
# Override BASE_IMAGE to point at a registry copy if needed.
ARG BASE_IMAGE=miaozao/bun-base:1.3.6
FROM ${BASE_IMAGE}

USER root
WORKDIR /app
RUN chown bun:bun /app
USER bun
COPY --chown=bun:bun package.json bun.lock ./
RUN bun install --frozen-lockfile --production
COPY --chown=bun:bun src ./src
COPY --chown=bun:bun public ./public
RUN mkdir -p /app/data/files /app/data/extracted
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=41874
ENV MONGODB_URI=mongodb://mongo:27017
ENV MONGODB_DB=agent_native_runtime
EXPOSE 41874
CMD ["bun", "src/server.js"]
