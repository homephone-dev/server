FROM golang:1.22 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/backend ./cmd/backend
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/backend /backend
COPY --from=build /out/migrate /migrate
COPY migrations /migrations
WORKDIR /
ENTRYPOINT ["/backend"]
