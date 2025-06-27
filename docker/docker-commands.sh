#!/bin/bash

# docker-commands.sh
# Useful Docker commands for managing the MySQL setup

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

echo_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

echo_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Start services
start_services() {
    echo_info "Starting MySQL services..."
    docker-compose up -d
    echo_info "Services started. MySQL: localhost:3306, phpMyAdmin: localhost:8080, Adminer: localhost:8081"
}

# Stop services
stop_services() {
    echo_info "Stopping MySQL services..."
    docker-compose down
    echo_info "Services stopped."
}

# Restart services
restart_services() {
    echo_info "Restarting MySQL services..."
    docker-compose restart
    echo_info "Services restarted."
}

# View logs
view_logs() {
    echo_info "Viewing MySQL logs..."
    docker-compose logs -f mysql
}

# Access MySQL shell
mysql_shell() {
    echo_info "Accessing MySQL shell..."
    docker-compose exec mysql mysql -u root -p loan_system
}

# Access MySQL shell as app user
mysql_shell_user() {
    echo_info "Accessing MySQL shell as loan_user..."
    docker-compose exec mysql mysql -u loan_user -p loan_system
}

# Backup database
backup_database() {
    BACKUP_FILE="backup_$(date +%Y%m%d_%H%M%S).sql"
    echo_info "Creating database backup: $BACKUP_FILE"
    docker-compose exec mysql mysqldump -u root -p loan_system >$BACKUP_FILE
    echo_info "Backup created: $BACKUP_FILE"
}

# Restore database
restore_database() {
    if [ -z "$1" ]; then
        echo_error "Please provide backup file name"
        echo "Usage: restore_database <backup_file.sql>"
        return 1
    fi

    echo_info "Restoring database from: $1"
    docker-compose exec -T mysql mysql -u root -p loan_system <$1
    echo_info "Database restored from: $1"
}

# Show database status
show_status() {
    echo_info "Database Status:"
    docker-compose ps
    echo
    echo_info "MySQL Process List:"
    docker-compose exec mysql mysql -u root -p -e "SHOW PROCESSLIST;"
}

# Clean up (remove containers and volumes)
cleanup() {
    echo_warn "This will remove all containers and data volumes!"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo_info "Removing containers and volumes..."
        docker-compose down -v
        docker volume prune -f
        echo_info "Cleanup completed."
    else
        echo_info "Cleanup cancelled."
    fi
}

# Initialize fresh database
init_fresh() {
    echo_info "Initializing fresh database..."
    docker-compose down -v
    docker-compose up -d
    echo_info "Waiting for MySQL to be ready..."
    sleep 30
    echo_info "Fresh database initialized."
}

# Check database connection
check_connection() {
    echo_info "Checking database connection..."
    docker-compose exec mysql mysqladmin -u root -p ping
}

# Show help
show_help() {
    echo "Available commands:"
    echo "  start          - Start all services"
    echo "  stop           - Stop all services"
    echo "  restart        - Restart all services"
    echo "  logs           - View MySQL logs"
    echo "  shell          - Access MySQL shell as root"
    echo "  shell-user     - Access MySQL shell as loan_user"
    echo "  backup         - Backup database"
    echo "  restore <file> - Restore database from backup"
    echo "  status         - Show database status"
    echo "  cleanup        - Remove containers and volumes"
    echo "  init-fresh     - Initialize fresh database"
    echo "  check          - Check database connection"
    echo "  help           - Show this help"
}

# Main command handler
case "$1" in
"start")
    start_services
    ;;
"stop")
    stop_services
    ;;
"restart")
    restart_services
    ;;
"logs")
    view_logs
    ;;
"shell")
    mysql_shell
    ;;
"shell-user")
    mysql_shell_user
    ;;
"backup")
    backup_database
    ;;
"restore")
    restore_database "$2"
    ;;
"status")
    show_status
    ;;
"cleanup")
    cleanup
    ;;
"init-fresh")
    init_fresh
    ;;
"check")
    check_connection
    ;;
"help" | "")
    show_help
    ;;
*)
    echo_error "Unknown command: $1"
    show_help
    ;;
esac
