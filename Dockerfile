FROM oven/bun:1.3.6-debian
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
