FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY proto ./proto
RUN CGO_ENABLED=0 go build -trimpath -o /out/pii-redactor ./cmd

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/pii-redactor /pii-redactor

EXPOSE 8080
ENTRYPOINT ["/pii-redactor"]
