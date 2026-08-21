FROM node:22-bookworm-slim
WORKDIR /app
COPY package*.json ./
RUN npm ci --omit=dev
COPY src ./src
COPY public ./public
RUN mkdir -p /app/data/uploads
ENV NODE_ENV=production
ENV HOST=0.0.0.0
ENV PORT=41873
ENV MONGODB_URI=mongodb://mongo:27017
ENV MONGODB_DB=agent_native_runtime
EXPOSE 41873
CMD ["npm", "start"]
