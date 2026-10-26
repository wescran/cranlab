FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /site ./cmd/site

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /site /site
ENTRYPOINT ["/site"]
