FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -tags containers_image_openpgp \
    -ldflags "-s -w" \
    -o /out/bin/catalog-bundle-inspect ./cmd/catalog-bundle-inspect && \
    CGO_ENABLED=0 go build -tags containers_image_openpgp \
    -ldflags "-s -w" \
    -o /out/bin/batch-validate ./cmd/batch-validate && \
    CGO_ENABLED=0 go build -tags containers_image_openpgp \
    -ldflags "-s -w" \
    -o /out/bin/telco-diagnose ./cmd/telco-diagnose && \
    CGO_ENABLED=0 go build -tags containers_image_openpgp \
    -ldflags "-s -w" \
    -o /out/bin/opm-diagnose ./cmd/opm-diagnose

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/bin/ /usr/local/bin/

USER nonroot:nonroot

ENTRYPOINT ["catalog-bundle-inspect"]
