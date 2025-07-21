# Этап сборки
FROM golang:1.24-alpine AS builder

# Устанавливаем зависимости для SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./

# Скачиваем зависимости (кэшируется Docker)
RUN go mod download && go mod verify

# Копируем исходный код
COPY . .

# Собираем приложение (ОПТИМИЗИРОВАННАЯ СБОРКА)
RUN CGO_ENABLED=1 GOOS=linux go build \
    -ldflags='-w -s -extldflags "-static"' \
    -tags sqlite_omit_load_extension \
    -o geocaching-bot .

# Финальный этап
FROM alpine:latest

# Устанавливаем зависимости для запуска
RUN apk --no-cache add ca-certificates sqlite tzdata

# Создаем пользователя для запуска приложения
RUN adduser -D -s /bin/sh geocaching

# Устанавливаем рабочую директорию
WORKDIR /app

# Создаем необходимые директории
RUN mkdir -p /app/photos /app/data && \
    chown -R geocaching:geocaching /app

# Копируем собранное приложение
COPY --from=builder /app/geocaching-bot .

# Устанавливаем права на выполнение
RUN chmod +x geocaching-bot

# Переключаемся на непривилегированного пользователя
USER geocaching

# Открываем порт (если понадобится для мониторинга)
EXPOSE 8080

# Запускаем приложение
CMD ["./geocaching-bot"] 