FROM node:16 as SPA
WORKDIR /usr/src/web

COPY web/package.json ./
COPY web/yarn.lock ./
RUN yarn install

COPY web ./

RUN yarn build

FROM gocv/opencv:4.5.3 as API

RUN apt-get update -qq
ENV TESSDATA_PREFIX=/usr/share/tesseract-ocr/4.00/tessdata/
RUN apt-get update -qq && apt-get install -y -qq libtesseract-dev libleptonica-dev \
    tesseract-ocr-eng libzbar-dev && apt-get clean autoclean \
    && apt-get autoremove --yes &&rm -rf /var/lib/{apt,dpkg,cache,log}/

WORKDIR /usr/src/api
ADD go.* ./
RUN go mod download
ADD . ./
COPY --from=SPA /usr/src/web/build ./web/build
RUN go build -o /build/bin/server ./cmd/server

EXPOSE 8080
ENTRYPOINT ["/build/bin/server"]