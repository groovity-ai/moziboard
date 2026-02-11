FROM node:20-alpine

WORKDIR /app

COPY package.json package-lock.json tsconfig.json ./
RUN npm ci

COPY src ./src
RUN npm run build

# Copy .env if exists, though better handled by docker-compose
# COPY .env .env

EXPOSE 3005

CMD ["node", "dist/index.js"]
