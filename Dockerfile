FROM golang:1.19-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

COPY internal /app/internal
COPY cmd /app/cmd
COPY vendor /app/vendor

RUN go build -o /app/app /app/cmd/orchestrator/main.go

FROM golang:1.19-alpine

COPY --from=build /app/app /app/app

CMD [ "/app/app" ]