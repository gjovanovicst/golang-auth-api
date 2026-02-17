#!/bin/bash
set -e

# Configuration
DB_CONTAINER="auth_db"
DB_USER="postgres"
DB_NAME="auth_db"

echo "Checking migration status..."

# 1. Ensure schema_migrations table exists
# We do this silently so we can query it later
docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "
CREATE TABLE IF NOT EXISTS schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    success BOOLEAN DEFAULT TRUE,
    execution_time_ms INTEGER,
    error_message TEXT,
    checksum VARCHAR(64)
);" > /dev/null 2>&1 || true

# 2. Get list of applied migrations from DB
# We use || true to handle cases where the table might not exist yet (though step 1 covers this)
APPLIED=$(docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -t -c "SELECT version FROM schema_migrations" 2>/dev/null || echo "")

# 3. Iterate over all .sql files in migrations directory, sorted by name
# using sort to ensure 00_ runs before 2024_ runs before 2026_
for file in $(ls migrations/*.sql | sort); do
    # Skip rollback files
    if [[ $file == *"_rollback.sql" ]]; then
        continue
    fi
    
    filename=$(basename "$file" .sql)
    version="$filename"
    
    # 4. Check if this version is in the APPLIED list
    if echo "$APPLIED" | grep -q "$version"; then
        # echo "Skipping $version (already applied)"
        continue
    fi
    
    echo "Applying migration: $version"
    
    # 5. Run the migration
    # We pipe the file content into docker exec psql
    if docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -v ON_ERROR_STOP=1 < "$file"; then
        # 6. Record success
        # We manually insert because not all SQL files contain the INSERT statement
        docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "
            INSERT INTO schema_migrations (version, name, success, applied_at)
            VALUES ('$version', '$version', true, NOW())
        ON CONFLICT (version) DO NOTHING;" > /dev/null
        echo "✅ Applied $version"
    else
        echo "❌ Failed to apply $version"
        exit 1
    fi
done

echo "All migrations up to date."
    fi

    echo "Applying migration: $version"
    
    # 5. Run the migration
    # We pipe the file content into docker exec psql
    if docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -v ON_ERROR_STOP=1 < "$file"; then
        # 6. Record success
        # We manually insert because not all SQL files contain the INSERT statement
        docker exec -i $DB_CONTAINER psql -U $DB_USER -d $DB_NAME -c "
            INSERT INTO schema_migrations (version, name, success, applied_at) 
            VALUES ('$version', '$version', true, NOW())
            ON CONFLICT (version) DO NOTHING;" > /dev/null
        echo "✅ Applied $version"
    else
        echo "❌ Failed to apply $version"
        exit 1
    fi
done

echo "All migrations up to date."
