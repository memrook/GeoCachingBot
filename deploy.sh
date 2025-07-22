#!/bin/bash

# GeoCaching Bot - Deploy Script
# Использование: ./deploy.sh [build|start|stop|restart|logs|status]

set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Функция для вывода сообщений
log() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Проверяем наличие .env файла
check_env() {
    if [ ! -f ".env" ]; then
        error "Файл .env не найден!"
        echo "Скопируйте env.example в .env и настройте переменные:"
        echo "cp env.example .env"
        echo "nano .env"
        echo ""
        echo "Обязательно настройте:"
        echo "  BOT_TOKEN=ваш_токен_от_BotFather"
        echo "  ADMIN_ID=ваш_telegram_user_id"
        exit 1
    fi
    
    # Проверяем основные переменные
    if ! grep -q "BOT_TOKEN=" .env || ! grep -q "^BOT_TOKEN=.*[^[:space:]]" .env; then
        error "BOT_TOKEN не настроен в .env файле!"
        echo "Получите токен от @BotFather и добавьте в .env:"
        echo "BOT_TOKEN=ваш_токен_бота"
        exit 1
    fi
    
    if ! grep -q -E "(ADMIN_ID=|ADMIN_IDS=)" .env; then
        error "ADMIN_ID или ADMIN_IDS не настроены в .env файле!"
        echo "Получите ваш User ID от @userinfobot и добавьте в .env:"
        echo "ADMIN_ID=ваш_telegram_user_id"
        exit 1
    fi
    
    log "✅ Конфигурация .env файла выглядит корректно"
}

# Быстрая настройка .env файла
setup_env() {
    if [ -f ".env" ]; then
        warn "Файл .env уже существует!"
        read -p "Перезаписать? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            log "Настройка отменена."
            return
        fi
    fi
    
    log "Создание .env файла из шаблона..."
    cp env.example .env
    
    echo
    echo "📝 Настройте следующие параметры в .env файле:"
    echo
    echo "1. BOT_TOKEN - получите от @BotFather:"
    echo "   https://t.me/BotFather → /newbot"
    echo
    echo "2. ADMIN_ID - получите от @userinfobot:"
    echo "   https://t.me/userinfobot → отправьте любое сообщение"
    echo
    
    if command -v nano >/dev/null 2>&1; then
        read -p "Открыть .env в редакторе nano? (Y/n): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Nn]$ ]]; then
            nano .env
        fi
    else
        echo "Отредактируйте файл .env любым текстовым редактором"
    fi
    
    log "✅ Файл .env создан! Теперь можно запустить бота: ./deploy.sh start"
}

# Создаем необходимые директории
create_dirs() {
    log "Создание необходимых директорий..."
    mkdir -p data logs
    chmod 755 data logs
}

# Сборка образа
build() {
    local dockerfile=${1:-"Dockerfile"}
    local no_cache=${2:-""}
    
    log "Сборка Docker образа с $dockerfile..."
    
    if [ "$no_cache" = "--no-cache" ]; then
        DOCKERFILE=$dockerfile docker-compose build --no-cache
    else
        DOCKERFILE=$dockerfile docker-compose build
    fi
    
    log "Образ успешно собран!"
}

# Быстрая сборка
build_fast() {
    log "Быстрая сборка с кэшированием..."
    build "Dockerfile" ""
}

# Оптимизированная сборка
build_optimized() {
    log "Оптимизированная сборка..."
    build "Dockerfile.optimized" ""
}

# Запуск сервисов
start() {
    check_env
    create_dirs
    log "Запуск GeoCaching Bot..."
    docker-compose up -d
    log "Бот запущен!"
    status
}

# Остановка сервисов
stop() {
    log "Остановка GeoCaching Bot..."
    docker-compose down
    log "Бот остановлен!"
}

# Перезапуск сервисов
restart() {
    log "Перезапуск GeoCaching Bot..."
    docker-compose restart
    log "Бот перезапущен!"
    status
}

# Просмотр логов
logs() {
    echo -e "${BLUE}=== Логи GeoCaching Bot ===${NC}"
    docker-compose logs -f --tail=100 geocaching-bot
}

# Статус сервисов
status() {
    echo -e "${BLUE}=== Статус сервисов ===${NC}"
    docker-compose ps
    
    echo -e "\n${BLUE}=== Использование ресурсов ===${NC}"
    docker stats --no-stream --format "table {{.Container}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}" geocaching-bot 2>/dev/null || true
}

# Обновление (пересборка + перезапуск)
update() {
    log "Обновление GeoCaching Bot..."
    docker-compose down
    build
    start
    log "Обновление завершено!"
}

# Очистка (остановка + удаление образов)
clean() {
    warn "Это удалит все образы и контейнеры GeoCaching Bot!"
    read -p "Продолжить? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log "Очистка Docker ресурсов..."
        docker-compose down -v --rmi all
        docker system prune -f
        log "Очистка завершена!"
    else
        log "Очистка отменена."
    fi
}

# Бэкап данных
backup() {
    BACKUP_DIR="backups/$(date +%Y%m%d_%H%M%S)"
    log "Создание бэкапа в $BACKUP_DIR..."
    
    mkdir -p "$BACKUP_DIR"
    
    # Бэкап базы данных
    if [ -d "data" ]; then
        cp -r data "$BACKUP_DIR/"
        log "База данных скопирована"
    fi
    

    
    # Бэкап конфигурации
    if [ -f ".env" ]; then
        cp .env "$BACKUP_DIR/"
        log "Конфигурация скопирована"
    fi
    
    log "Бэкап создан в $BACKUP_DIR"
}

# Основная логика
case "${1:-}" in
    build)
        build "Dockerfile" "--no-cache"
        ;;
    build-fast)
        build_fast
        ;;
    build-optimized)
        build_optimized
        ;;
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        restart
        ;;
    logs)
        logs
        ;;
    status)
        status
        ;;
    update)
        update
        ;;
    clean)
        clean
        ;;
    backup)
        backup
        ;;
    setup)
        setup_env
        ;;
    *)
        echo -e "${BLUE}GeoCaching Bot - Deploy Script${NC}"
        echo
        echo "Использование: $0 [команда]"
        echo
        echo "Доступные команды:"
        echo "  build              - Собрать Docker образ (полная пересборка)"
        echo "  build-fast         - Быстрая сборка с кэшированием"
        echo "  build-optimized    - Оптимизированная сборка (scratch образ)"
        echo "  start              - Запустить бота"
        echo "  stop     - Остановить бота"
        echo "  restart  - Перезапустить бота"
        echo "  logs     - Показать логи"
        echo "  status   - Показать статус"
        echo "  update   - Обновить (пересборка + перезапуск)"
        echo "  backup   - Создать бэкап данных"
        echo "  setup    - Быстрая настройка .env файла"
        echo "  clean    - Полная очистка (ОСТОРОЖНО!)"
        echo
        echo "Примеры:"
        echo "  $0 start     # Запуск бота"
        echo "  $0 logs      # Просмотр логов"
        echo "  $0 status    # Проверка статуса"
        ;;
esac 