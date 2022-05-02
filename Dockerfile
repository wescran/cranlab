FROM node:18
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm install @11ty/eleventy
ENTRYPOINT ["npm","start"]
