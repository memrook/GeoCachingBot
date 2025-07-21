# Этап сборки
FROM golang:1.21-alpine AS builder

# Устанавливаем зависимости для SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем исходный код
COPY . .

# Собираем приложение
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o geocaching-bot .

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