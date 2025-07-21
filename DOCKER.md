# 🐳 Docker деплой GeoCaching Bot

Полноценное руководство по развертыванию GeoCaching Bot с использованием Docker и Docker Compose.

## 🚀 Быстрый старт

### 1. Подготовка
```bash
# Клонируйте репозиторий
git clone <repository-url>
cd GeoCachingBot

# Скопируйте и настройте конфигурацию
cp env.example .env
nano .env  # Настройте BOT_TOKEN и ADMIN_ID

# Запустите бота
./deploy.sh start
```

### 2. Проверка статуса
```bash
./deploy.sh status
./deploy.sh logs
```

## 📋 Требования

- **Docker** 20.10+
- **Docker Compose** 2.0+
- **Bash** (для скрипта deploy.sh)

## 🛠️ Команды управления

### Основные команды
```bash
./deploy.sh start     # 🚀 Запуск бота
./deploy.sh stop      # 🛑 Остановка бота
./deploy.sh restart   # 🔄 Перезапуск бота
./deploy.sh status    # 📊 Статус и ресурсы
./deploy.sh logs      # 📜 Просмотр логов
```

### Операции с данными
```bash
./deploy.sh backup    # 💾 Создание бэкапа
./deploy.sh update    # ⬆️ Обновление бота
./deploy.sh clean     # 🧹 Полная очистка (ОСТОРОЖНО!)
```

### Разработка
```bash
./deploy.sh build     # 🔨 Сборка образа
```

## 📁 Структура файлов

```
GeoCachingBot/
├── docker-compose.yml    # Конфигурация сервисов
├── Dockerfile            # Образ приложения
├── deploy.sh             # Скрипт управления
├── .dockerignore         # Исключения для сборки
├── .env                  # Переменные окружения
├── data/                 # База данных SQLite
├── photos/               # Фотографии тайников
├── logs/                 # Логи приложения
└── backups/              # Бэкапы данных
```

## 🔧 Конфигурация

### Основные переменные (.env)
```bash
# Обязательные
BOT_TOKEN=ваш_токен_бота
ADMIN_ID=ваш_user_id

# Пути (настроены для Docker)
DATABASE_PATH=/app/data/geocaching.db
PHOTO_STORAGE_PATH=/app/photos

# Настройки геокэшинга
TARGET_DISTANCE_METERS=200
LIVE_LOCATION_DURATION_HOURS=1
```

### Docker Compose конфигурация
- **Автоперезапуск**: `restart: unless-stopped`
- **Лимиты ресурсов**: 256MB RAM, 0.5 CPU
- **Healthcheck**: Проверка каждые 30 секунд
- **Логирование**: Ротация по 10MB, 3 файла

## 💾 Управление данными

### Volumes
- `./data` → `/app/data` - База данных SQLite
- `./photos` → `/app/photos` - Фотографии тайников
- `./logs` → `/app/logs` - Логи приложения

### Бэкапы
```bash
# Автоматический бэкап
./deploy.sh backup

# Ручной бэкап
cp -r data photos .env backups/manual_$(date +%Y%m%d)
```

### Восстановление
```bash
# Остановить бота
./deploy.sh stop

# Восстановить данные
cp -r backups/20241221_120000/data ./
cp -r backups/20241221_120000/photos ./

# Запустить бота
./deploy.sh start
```

## 📊 Мониторинг

### Просмотр статуса
```bash
./deploy.sh status
```

Выводит:
- Статус контейнера
- Использование CPU и памяти
- Сетевую активность

### Просмотр логов
```bash
# Последние 100 строк с follow
./deploy.sh logs

# Логи за определенный период
docker-compose logs --since=1h geocaching-bot

# Логи с фильтрацией
docker-compose logs geocaching-bot | grep ERROR
```

### Healthcheck
Автоматическая проверка состояния каждые 30 секунд:
- ✅ Healthy - процесс работает
- ❌ Unhealthy - процесс не отвечает

## 🔒 Безопасность

### Рекомендации
- ✅ Бот работает под непривилегированным пользователем
- ✅ .env файл исключен из Docker образа
- ✅ Данные хранятся в защищенных volumes
- ✅ Лимиты ресурсов предотвращают DoS

### Файервол (опционально)
```bash
# Если нужны внешние порты
sudo ufw allow 8080/tcp  # Только если планируется веб-интерфейс
```

## 🚨 Решение проблем

### Бот не запускается
```bash
# Проверьте логи
./deploy.sh logs

# Проверьте конфигурацию
cat .env | grep -v "^#"

# Пересоберите образ
./deploy.sh build
./deploy.sh start
```

### Ошибки доступа к файлам
```bash
# Проверьте права доступа
ls -la data/ photos/

# Исправьте права (если нужно)
chmod 755 data photos
```

### Проблемы с памятью
```bash
# Проверьте использование ресурсов
./deploy.sh status

# Увеличьте лимиты в docker-compose.yml
# memory: 512M  # было 256M
```

### Очистка и переустановка
```bash
# Полная очистка (сохранит данные)
./deploy.sh clean

# Заново сборка и запуск
./deploy.sh build
./deploy.sh start
```

## 📈 Масштабирование

### Горизонтальное масштабирование
Если нужно обрабатывать больше пользователей:

```yaml
# docker-compose.yml
services:
  geocaching-bot:
    scale: 2  # Добавить в deploy секцию
```

⚠️ **Внимание**: SQLite не поддерживает параллельную запись из нескольких процессов!

### Мониторинг производительности
```bash
# Мониторинг в реальном времени
watch -n 5 "docker stats geocaching-bot --no-stream"

# Логи производительности
docker-compose logs geocaching-bot | grep -E "(время|memory|cpu)"
```

## 🔄 CI/CD интеграция

### GitHub Actions пример
```yaml
# .github/workflows/deploy.yml
name: Deploy Bot
on:
  push:
    branches: [main]
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Deploy to server
        run: |
          ssh user@server "cd /path/to/bot && git pull && ./deploy.sh update"
```

## 📞 Поддержка

При проблемах с Docker деплоем:

1. 📋 Соберите логи: `./deploy.sh logs > logs.txt`
2. 📊 Проверьте статус: `./deploy.sh status`
3. 🔍 Проверьте конфигурацию: `cat .env`
4. 💾 Создайте бэкап: `./deploy.sh backup`

---

**🎯 Готово! Ваш GeoCaching Bot работает в Docker!** 🎉 