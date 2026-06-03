FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /out/protoradar-server ./cmd/protoradar-server

FROM bufbuild/buf:1.55.1 AS buf

FROM alpine:3.22

WORKDIR /app
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
COPY --from=build /out/protoradar-server /usr/local/bin/protoradar-server
COPY migrations ./migrations

EXPOSE 8080
ENTRYPOINT ["protoradar-server"]
