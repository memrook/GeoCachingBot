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
        exit 1
    fi
}

# Создаем необходимые директории
create_dirs() {
    log "Создание необходимых директорий..."
    mkdir -p data photos logs
    chmod 755 data photos logs
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
    
    # Бэкап фотографий
    if [ -d "photos" ]; then
        cp -r photos "$BACKUP_DIR/"
        log "Фотографии скопированы"
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
        echo "  clean    - Полная очистка (ОСТОРОЖНО!)"
        echo
        echo "Примеры:"
        echo "  $0 start     # Запуск бота"
        echo "  $0 logs      # Просмотр логов"
        echo "  $0 status    # Проверка статуса"
        ;;
esac 