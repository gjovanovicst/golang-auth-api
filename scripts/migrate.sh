#!/bin/bash

# Database Migration Script
# Interactive tool for managing database migrations

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Database connection (can be overridden by environment)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5433}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-auth_db}"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Database Migration Tool${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo -e "${RED}Error: psql command not found. Please install PostgreSQL client.${NC}"
    exit 1
fi

# Function to execute SQL and show results
execute_sql() {
    local sql="$1"
    local description="$2"
    
    echo -e "${YELLOW}$description${NC}"
    echo ""
    
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "$sql"
    
    echo ""
}

# Function to execute SQL file
execute_sql_file() {
    local file="$1"
    local description="$2"
    
    echo -e "${YELLOW}$description${NC}"
    echo ""
    
    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$file"
    
    echo ""
}

# Function to confirm action
confirm() {
    local prompt="$1"
    echo -e "${YELLOW}$prompt (yes/no): ${NC}"
    read -r response
    if [[ "$response" != "yes" ]]; then
        echo -e "${RED}Operation cancelled.${NC}"
        return 1
    fi
    return 0
}

# Apply multi-tenancy migration
apply_mt_migration() {
    echo -e "${GREEN}Applying Multi-Tenancy Migration (v1.2.0)${NC}"
    echo ""
    
    local migration_file="migrations/20260105_add_multi_tenancy.sql"
    
    if [ ! -f "$migration_file" ]; then
        echo -e "${RED}Error: Migration file not found!${NC}"
        echo "Expected: $migration_file"
        exit 1
    fi
    
    echo "This will:"
    echo "- Create tenants, applications, oauth_provider_configs tables"
    echo "- Add default tenant and app"
    echo "- Add app_id to users, social_accounts, activity_logs"
    echo "- Migrate existing data to default app"
    echo ""
    
    if ! confirm "Apply migration?"; then
        return
    fi
    
    execute_sql_file "$migration_file" "Applying migration..."
}

# Rollback multi-tenancy migration
rollback_mt_migration() {
    echo -e "${GREEN}Rollback Multi-Tenancy Migration${NC}"
    echo ""
    
    local rollback_file="migrations/20260105_add_multi_tenancy_rollback.sql"
    
    if [ ! -f "$rollback_file" ]; then
        echo -e "${RED}Error: Rollback file not found!${NC}"
        echo "Expected: $rollback_file"
        exit 1
    fi
    
    echo -e "${RED}WARNING: This will drop tenants, applications tables and remove app_id columns!${NC}"
    echo ""
    
    if ! confirm "Are you sure you want to rollback?"; then
        return
    fi
    
    execute_sql_file "$rollback_file" "Rolling back..."
}

# Show current migration status
show_status() {
    echo -e "${GREEN}Current Database Status:${NC}"
    echo ""
    
    # Check if activity_logs table exists
    execute_sql "
        SELECT EXISTS (
            SELECT FROM information_schema.tables
            WHERE table_schema = 'public'
            AND table_name = 'activity_logs'
        ) as activity_logs_exists;
    " "Checking tables..."
    
    # Check if smart logging fields exist
    execute_sql "
        SELECT column_name, data_type
        FROM information_schema.columns
        WHERE table_name = 'activity_logs'
        AND column_name IN ('severity', 'expires_at', 'is_anomaly')
        ORDER BY column_name;
    " "Checking smart logging fields..."
    
    # Check indexes
    execute_sql "
        SELECT indexname
        FROM pg_indexes
        WHERE tablename = 'activity_logs'
        AND indexname LIKE '%cleanup%' OR indexname LIKE '%expires%'
        ORDER BY indexname;
    " "Checking smart logging indexes..."
    
    # Count logs by type
    execute_sql "
        SELECT
            COUNT(*) as total_logs,
            pg_size_pretty(pg_total_relation_size('activity_logs')) as table_size
        FROM activity_logs;
    " "Counting logs..."
}

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Migration Options:${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""
echo "1. Show migration status"
echo "2. Apply smart logging migration (v1.1.0)"
echo "3. Rollback smart logging migration"
echo "4. List all available migrations"
echo "5. Backup database"
echo "6. Test database connection"
echo "0. Exit"
echo ""
echo -e "${YELLOW}Enter your choice (0-6): ${NC}"
read -r choice

case $choice in
    1)
        show_status
    ;;
    
    2)
        echo -e "${GREEN}Applying Smart Logging Migration (v1.1.0)${NC}"
        echo ""
        
        if [ ! -f "migrations/20240103_add_activity_log_smart_fields.sql" ]; then
            echo -e "${RED}Error: Migration file not found!${NC}"
            echo "Expected: migrations/20240103_add_activity_log_smart_fields.sql"
            exit 1
        fi
        
        echo -e "${YELLOW}This will:${NC}"
        echo "- Add severity field to activity_logs"
        echo "- Add expires_at field to activity_logs"
        echo "- Add is_anomaly field to activity_logs"
        echo "- Create new indexes"
        echo "- Update existing logs with defaults"
        echo ""
        
        if confirm "Apply migration?"; then
            echo -e "${GREEN}Creating backup first...${NC}"
            timestamp=$(date +%Y%m%d_%H%M%S)
            backup_file="backup_before_smart_logging_${timestamp}.sql"
            
            PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" > "$backup_file"
            echo -e "${GREEN}Backup saved to: $backup_file${NC}"
            echo ""
            
            execute_sql_file "migrations/20240103_add_activity_log_smart_fields.sql" "Applying migration..."
            
            echo -e "${GREEN}Migration completed successfully!${NC}"
            echo ""
            
            show_status
        fi
    ;;
    
    3)
        echo -e "${RED}Rollback Smart Logging Migration${NC}"
        echo ""
        
        if [ ! -f "migrations/20240103_add_activity_log_smart_fields_rollback.sql" ]; then
            echo -e "${RED}Error: Rollback file not found!${NC}"
            exit 1
        fi
        
        echo -e "${YELLOW}WARNING: This will remove smart logging fields!${NC}"
        echo ""
        
        if confirm "Are you sure you want to rollback?"; then
            echo -e "${GREEN}Creating backup first...${NC}"
            timestamp=$(date +%Y%m%d_%H%M%S)
            backup_file="backup_before_rollback_${timestamp}.sql"
            
            PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" > "$backup_file"
            echo -e "${GREEN}Backup saved to: $backup_file${NC}"
            echo ""
            
            execute_sql_file "migrations/20240103_add_activity_log_smart_fields_rollback.sql" "Rolling back..."
            
            echo -e "${GREEN}Rollback completed!${NC}"
            echo ""
            
            show_status
        fi
    ;;
    
    4) apply_mt_migration ;;
    5) rollback_mt_migration ;;
    6)
        echo -e "${GREEN}Available Migrations:${NC}"
        echo ""
        
        if [ -d "migrations" ]; then
            echo "Migrations in migrations/ directory:"
            echo ""
            ls -1 migrations/*.sql 2>/dev/null || echo "No migration files found"
        else
            echo -e "${RED}Migrations directory not found!${NC}"
        fi
    ;;
    
    7)
        echo -e "${GREEN}Creating Database Backup${NC}"
        echo ""
        
        timestamp=$(date +%Y%m%d_%H%M%S)
        backup_file="backup_${DB_NAME}_${timestamp}.sql"
        
        echo -e "${YELLOW}Backing up to: $backup_file${NC}"
        
        PGPASSWORD="$DB_PASSWORD" pg_dump -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" > "$backup_file"
        
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}Backup created successfully!${NC}"
            echo "File: $backup_file"
            echo "Size: $(du -h "$backup_file" | cut -f1)"
        else
            echo -e "${RED}Backup failed!${NC}"
            exit 1
        fi
    ;;
    
    8)
        echo -e "${GREEN}Testing Database Connection${NC}"
        echo ""
        
        echo "Host: $DB_HOST"
        echo "Port: $DB_PORT"
        echo "User: $DB_USER"
        echo "Database: $DB_NAME"
        echo ""
        
        if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT version();" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Connection successful!${NC}"
            echo ""
            
            execute_sql "SELECT version();" "PostgreSQL Version:"
        else
            echo -e "${RED}✗ Connection failed!${NC}"
            echo ""
            echo "Please check:"
            echo "- Database is running"
            echo "- Host and port are correct"
            echo "- Username and password are correct"
            echo "- Database exists"
            exit 1
        fi
    ;;
    
    0)
        echo -e "${GREEN}Exiting...${NC}"
        exit 0
    ;;
    
    *)
        echo -e "${RED}Invalid choice. Exiting.${NC}"
        exit 1
    ;;
esac

echo ""
echo -e "${BLUE}======================================${NC}"
echo -e "${GREEN}Operation completed!${NC}"
echo -e "${BLUE}======================================${NC}"

