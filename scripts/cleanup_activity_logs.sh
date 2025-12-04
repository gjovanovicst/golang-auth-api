#!/bin/bash

# Activity Logs Cleanup Script (Bash wrapper)
# This script provides a safe, interactive way to clean up activity logs

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Database connection variables (can be overridden by environment)
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_NAME="${DB_NAME:-auth_db}"

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Activity Logs Cleanup Utility${NC}"
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

# Show current statistics
echo -e "${GREEN}Current Activity Logs Statistics:${NC}"
echo ""

execute_sql "
SELECT 
    COUNT(*) as total_logs,
    pg_size_pretty(pg_total_relation_size('activity_logs')) as total_size
FROM activity_logs;

SELECT severity, COUNT(*) as count 
FROM activity_logs 
GROUP BY severity 
ORDER BY severity;
" "Database Statistics"

echo ""
echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}Cleanup Options:${NC}"
echo -e "${BLUE}======================================${NC}"
echo ""
echo "1. Delete ALL activity logs (DANGEROUS)"
echo "2. Delete logs older than 30 days"
echo "3. Delete logs older than 90 days"
echo "4. Delete expired logs only (follows retention policies)"
echo "5. Delete only INFORMATIONAL severity logs"
echo "6. Delete TOKEN_REFRESH and PROFILE_ACCESS logs"
echo "7. Delete logs for specific user (GDPR)"
echo "8. Keep only last 7 days"
echo "9. Show statistics only (no deletion)"
echo "0. Exit"
echo ""
echo -e "${YELLOW}Enter your choice (0-9): ${NC}"
read -r choice

case $choice in
    1)
        echo -e "${RED}WARNING: This will delete ALL activity logs!${NC}"
        if confirm "Are you absolutely sure you want to delete ALL logs?"; then
            if confirm "This cannot be undone. Type 'yes' again to confirm"; then
                echo -e "${GREEN}Deleting all logs...${NC}"
                execute_sql "DELETE FROM activity_logs;" "Deleting all logs"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}All logs deleted successfully.${NC}"
            fi
        fi
        ;;
    
    2)
        if confirm "Delete logs older than 30 days?"; then
            execute_sql "
                SELECT COUNT(*) as logs_to_delete 
                FROM activity_logs 
                WHERE timestamp < NOW() - INTERVAL '30 days';
            " "Logs that will be deleted"
            
            if confirm "Proceed with deletion?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE timestamp < NOW() - INTERVAL '30 days';
                " "Deleting logs older than 30 days"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}Cleanup completed.${NC}"
            fi
        fi
        ;;
    
    3)
        if confirm "Delete logs older than 90 days?"; then
            execute_sql "
                SELECT COUNT(*) as logs_to_delete 
                FROM activity_logs 
                WHERE timestamp < NOW() - INTERVAL '90 days';
            " "Logs that will be deleted"
            
            if confirm "Proceed with deletion?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE timestamp < NOW() - INTERVAL '90 days';
                " "Deleting logs older than 90 days"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}Cleanup completed.${NC}"
            fi
        fi
        ;;
    
    4)
        echo -e "${GREEN}This is the recommended option - follows retention policies${NC}"
        execute_sql "
            SELECT COUNT(*) as expired_logs 
            FROM activity_logs 
            WHERE expires_at < NOW();
        " "Expired logs that will be deleted"
        
        if confirm "Delete expired logs?"; then
            execute_sql "
                DELETE FROM activity_logs 
                WHERE expires_at < NOW();
            " "Deleting expired logs"
            execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
            echo -e "${GREEN}Cleanup completed.${NC}"
        fi
        ;;
    
    5)
        if confirm "Delete all INFORMATIONAL severity logs?"; then
            execute_sql "
                SELECT COUNT(*) as logs_to_delete 
                FROM activity_logs 
                WHERE severity = 'INFORMATIONAL';
            " "Informational logs that will be deleted"
            
            if confirm "Proceed with deletion?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE severity = 'INFORMATIONAL';
                " "Deleting informational logs"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}Cleanup completed.${NC}"
            fi
        fi
        ;;
    
    6)
        if confirm "Delete TOKEN_REFRESH and PROFILE_ACCESS logs?"; then
            execute_sql "
                SELECT event_type, COUNT(*) as count 
                FROM activity_logs 
                WHERE event_type IN ('TOKEN_REFRESH', 'PROFILE_ACCESS')
                GROUP BY event_type;
            " "Logs that will be deleted"
            
            if confirm "Proceed with deletion?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE event_type IN ('TOKEN_REFRESH', 'PROFILE_ACCESS');
                " "Deleting TOKEN_REFRESH and PROFILE_ACCESS logs"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}Cleanup completed.${NC}"
            fi
        fi
        ;;
    
    7)
        echo -e "${YELLOW}Enter user ID (UUID) to delete logs for: ${NC}"
        read -r user_id
        
        if confirm "Delete all logs for user $user_id? (GDPR compliance)"; then
            execute_sql "
                SELECT COUNT(*) as logs_to_delete 
                FROM activity_logs 
                WHERE user_id = '$user_id';
            " "Logs that will be deleted for this user"
            
            if confirm "Proceed with deletion?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE user_id = '$user_id';
                " "Deleting logs for user $user_id"
                echo -e "${GREEN}User logs deleted successfully.${NC}"
            fi
        fi
        ;;
    
    8)
        echo -e "${RED}WARNING: This will delete ALL logs except the last 7 days!${NC}"
        if confirm "Keep only last 7 days and delete everything else?"; then
            execute_sql "
                SELECT COUNT(*) as logs_to_delete 
                FROM activity_logs 
                WHERE timestamp < NOW() - INTERVAL '7 days';
            " "Logs that will be deleted"
            
            if confirm "This is a large deletion. Proceed?"; then
                execute_sql "
                    DELETE FROM activity_logs 
                    WHERE timestamp < NOW() - INTERVAL '7 days';
                " "Deleting old logs"
                execute_sql "VACUUM ANALYZE activity_logs;" "Reclaiming space"
                echo -e "${GREEN}Cleanup completed.${NC}"
            fi
        fi
        ;;
    
    9)
        echo -e "${GREEN}Detailed Statistics:${NC}"
        execute_sql "
            -- Total count and size
            SELECT 
                COUNT(*) as total_logs,
                pg_size_pretty(pg_total_relation_size('activity_logs')) as total_size
            FROM activity_logs;
            
            -- By severity
            SELECT severity, COUNT(*) as count 
            FROM activity_logs 
            GROUP BY severity 
            ORDER BY severity;
            
            -- By event type (top 10)
            SELECT event_type, COUNT(*) as count 
            FROM activity_logs 
            GROUP BY event_type 
            ORDER BY count DESC 
            LIMIT 10;
            
            -- Time range
            SELECT 
                MIN(timestamp) as oldest_log,
                MAX(timestamp) as newest_log
            FROM activity_logs;
            
            -- Expired logs
            SELECT COUNT(*) as expired_logs_ready_for_cleanup
            FROM activity_logs 
            WHERE expires_at < NOW();
        " "Complete Statistics"
        ;;
    
    0)
        echo -e "${GREEN}Exiting without changes.${NC}"
        exit 0
        ;;
    
    *)
        echo -e "${RED}Invalid choice. Exiting.${NC}"
        exit 1
        ;;
esac

echo ""
echo -e "${GREEN}Operation completed!${NC}"
echo ""
echo -e "${BLUE}Final Statistics:${NC}"
execute_sql "
    SELECT 
        COUNT(*) as remaining_logs,
        pg_size_pretty(pg_total_relation_size('activity_logs')) as current_size
    FROM activity_logs;
" "After cleanup"

