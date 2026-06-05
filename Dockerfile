FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
ARG VERSION_PKG=github.com/alryzden/ProtoRadar/internal/version
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.Commit=${COMMIT} -X ${VERSION_PKG}.BuildDate=${BUILD_DATE}" -o /out/protoradar-server ./cmd/protoradar-server && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X ${VERSION_PKG}.Version=${VERSION} -X ${VERSION_PKG}.Commit=${COMMIT} -X ${VERSION_PKG}.BuildDate=${BUILD_DATE}" -o /out/protoradar ./cmd/protoradar

FROM bufbuild/buf:1.55.1 AS buf

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app
COPY --from=buf /usr/local/bin/buf /usr/local/bin/buf
COPY --from=build /out/protoradar-server /usr/local/bin/protoradar-server
COPY --from=build /out/protoradar /usr/local/bin/protoradar
COPY migrations ./migrations

EXPOSE 8080
ENTRYPOINT ["protoradar-server"]
