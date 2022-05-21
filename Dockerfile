FROM node:18-alpine as build
WORKDIR /html
COPY package.json package-lock.json .eleventy.js ./
COPY src ./src
RUN npm install @11ty/eleventy && npm run build

FROM nginx:1.21
COPY --from=build /html/dist /usr/share/nginx/html
LABEL version="1.0.0"
LABEL maintaner="Wesley Cranston"
