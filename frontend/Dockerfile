# --- Build stage ---
FROM node:20.18-alpine AS build

WORKDIR /app

COPY package.json ./
RUN npm i -g npm@10.9.8 2>/dev/null
RUN npm install --no-audit --no-fund

COPY . .
RUN npm run build

# --- Production stage ---
FROM nginx:alpine
COPY --from=build /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
