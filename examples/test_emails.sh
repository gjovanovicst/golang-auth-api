#!/usr/bin/env bash
# =============================================================================
# Auth API - Email Testing Script
# =============================================================================
#
# Tests all 6 email types, SMTP configuration, and template preview via curl.
#
# Usage:
#   ./test_emails.sh                  # Run all email send tests
#   ./test_emails.sh --setup          # Configure SMTP first, then run tests
#   ./test_emails.sh --preview-only   # Only run template preview tests
#   ./test_emails.sh --send-only      # Only send emails (no previews)
#   ./test_emails.sh --smtp-test      # Only test SMTP connectivity
#   ./test_emails.sh --list           # List email types, templates, variables
#   ./test_emails.sh --all            # Run everything (setup + list + send + preview + SMTP test)
#   ./test_emails.sh --app-key        # Test app API key restrictions and access
#
# Environment variables (or edit the defaults below):
#   BASE_URL        - API base URL (default: http://localhost:8080)
#   ADMIN_API_KEY   - Admin API key
#   APP_API_KEY     - Per-application API key (for --app-key tests)
#   APP_ID          - Application UUID
#   CONFIG_ID       - SMTP config UUID (for --smtp-test with specific config)
#   TEST_EMAIL      - Email address to send test emails to
#
# In dev mode (no SMTP configured), emails are logged to the server console.
# =============================================================================

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration - edit these or set as environment variables
# ---------------------------------------------------------------------------
BASE_URL="${BASE_URL:-http://localhost:8080}"
ADMIN_API_KEY="${ADMIN_API_KEY:-your-admin-api-key-here}"
APP_API_KEY="${APP_API_KEY:-your-app-api-key-here}"
APP_ID="${APP_ID:-00000000-0000-0000-0000-000000000000}"
CONFIG_ID="${CONFIG_ID:-}"
TEST_EMAIL="${TEST_EMAIL:-test@example.com}"

# SMTP config for --setup (edit these for your provider)
SMTP_HOST="${SMTP_HOST:-smtp.gmail.com}"
SMTP_PORT="${SMTP_PORT:-587}"
SMTP_USERNAME="${SMTP_USERNAME:-your-email@gmail.com}"
SMTP_PASSWORD="${SMTP_PASSWORD:-your-app-password}"
SMTP_FROM_ADDRESS="${SMTP_FROM_ADDRESS:-your-email@gmail.com}"
SMTP_FROM_NAME="${SMTP_FROM_NAME:-My Auth App}"

# ---------------------------------------------------------------------------
# Colors and helpers
# ---------------------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

pass=0
fail=0

print_header() {
    echo ""
    echo -e "${BOLD}${BLUE}============================================================${NC}"
    echo -e "${BOLD}${BLUE}  $1${NC}"
    echo -e "${BOLD}${BLUE}============================================================${NC}"
    echo ""
}

print_section() {
    echo ""
    echo -e "${BOLD}${CYAN}--- $1 ---${NC}"
    echo ""
}

print_request() {
    echo -e "${YELLOW}>> $1${NC}"
}

check_response() {
    local description="$1"
    local http_code="$2"
    local body="$3"

    if [[ "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
        echo -e "${GREEN}  PASS${NC} ($http_code) $description"
        ((pass++))
    else
        echo -e "${RED}  FAIL${NC} ($http_code) $description"
        echo -e "${RED}  Response: $body${NC}"
        ((fail++))
    fi
}

# check_response_expect verifies a specific expected HTTP status code.
# Used for testing negative cases (e.g., 401, 403).
check_response_expect() {
    local description="$1"
    local expected_code="$2"
    local actual_code="$3"
    local body="$4"

    if [[ "$actual_code" -eq "$expected_code" ]]; then
        echo -e "${GREEN}  PASS${NC} ($actual_code) $description"
        ((pass++))
    else
        echo -e "${RED}  FAIL${NC} (expected $expected_code, got $actual_code) $description"
        echo -e "${RED}  Response: $body${NC}"
        ((fail++))
    fi
}

do_request() {
    local method="$1"
    local url="$2"
    local description="$3"
    local data="${4:-}"

    print_request "$method $url"

    local response
    local http_code

    if [[ -n "$data" ]]; then
        response=$(curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "X-Admin-API-Key: $ADMIN_API_KEY" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$url" 2>&1)
    else
        response=$(curl -s -w "\n%{http_code}" \
            -X "$method" \
            -H "X-Admin-API-Key: $ADMIN_API_KEY" \
            "$url" 2>&1)
    fi

    http_code=$(echo "$response" | tail -1)
    local body
    body=$(echo "$response" | sed '$d')

    check_response "$description" "$http_code" "$body"

    # Print abbreviated response for list operations
    if [[ "$method" == "GET" ]]; then
        echo -e "  ${CYAN}Response (first 200 chars):${NC} ${body:0:200}"
    fi
    echo ""
}

print_summary() {
    echo ""
    echo -e "${BOLD}============================================================${NC}"
    echo -e "${BOLD}  SUMMARY${NC}"
    echo -e "${BOLD}============================================================${NC}"
    echo -e "  ${GREEN}Passed: $pass${NC}"
    echo -e "  ${RED}Failed: $fail${NC}"
    echo -e "  Total:  $((pass + fail))"
    echo ""

    if [[ $fail -gt 0 ]]; then
        echo -e "${RED}Some tests failed. Check your configuration:${NC}"
        echo "  - Is the API server running at $BASE_URL?"
        echo "  - Is ADMIN_API_KEY correct?"
        echo "  - Is APP_ID a valid application UUID?"
        echo "  - Is SMTP configured? (for SMTP test failures)"
        return 1
    else
        echo -e "${GREEN}All tests passed!${NC}"
        return 0
    fi
}

# ---------------------------------------------------------------------------
# Test functions
# ---------------------------------------------------------------------------

setup_smtp() {
    print_header "SMTP Setup"

    do_request POST \
        "$BASE_URL/admin/email-servers?app_id=$APP_ID" \
        "Configure SMTP server ($SMTP_HOST:$SMTP_PORT)" \
        "{
            \"name\": \"Test SMTP Config\",
            \"smtp_host\": \"$SMTP_HOST\",
            \"smtp_port\": $SMTP_PORT,
            \"smtp_username\": \"$SMTP_USERNAME\",
            \"smtp_password\": \"$SMTP_PASSWORD\",
            \"from_address\": \"$SMTP_FROM_ADDRESS\",
            \"from_name\": \"$SMTP_FROM_NAME\",
            \"use_tls\": true,
            \"is_default\": true,
            \"is_active\": true
        }"
}

list_resources() {
    print_header "List Email Resources"

    print_section "Email Types"
    do_request GET "$BASE_URL/admin/email-types" "List all email types"

    print_section "Well-Known Variables"
    do_request GET "$BASE_URL/admin/email-variables" "List well-known variables"

    print_section "Global Default Templates"
    do_request GET "$BASE_URL/admin/email-templates" "List global default templates"

    print_section "App-Specific Templates"
    do_request GET "$BASE_URL/admin/email-templates?app_id=$APP_ID" "List app-specific templates"

    print_section "Email Servers"
    do_request GET "$BASE_URL/admin/email-servers" "List all SMTP server configs"
}

send_all_email_types() {
    print_header "Send All 6 Email Types"

    print_section "1. Email Verification"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send email_verification" \
        '{
            "type_code": "email_verification",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "verification_link": "https://example.com/verify-email?token=test-token-abc123",
                "verification_token": "test-token-abc123",
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "2. Password Reset"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send password_reset" \
        '{
            "type_code": "password_reset",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "reset_link": "https://example.com/reset-password?token=reset-xyz789",
                "expiration_minutes": "60",
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "3. 2FA Verification Code"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send two_fa_code" \
        '{
            "type_code": "two_fa_code",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "code": "847293",
                "expiration_minutes": "5",
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "4. Welcome"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send welcome" \
        '{
            "type_code": "welcome",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "5. Account Deactivated"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send account_deactivated" \
        '{
            "type_code": "account_deactivated",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "6. Password Changed"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/send-email" \
        "Send password_changed" \
        '{
            "type_code": "password_changed",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "change_time": "2026-02-25 14:30:00 UTC",
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'
}

preview_all_templates() {
    print_header "Preview All 6 Templates"

    print_section "1. Email Verification (go_template)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview email_verification template" \
        '{
            "subject": "Verify Your Email Address",
            "body_html": "<h2>Hello {{.UserName}}</h2><p>Please <a href=\"{{.VerificationLink}}\">verify your email</a> for {{.AppName}}.</p>",
            "body_text": "Hello {{.UserName}}, please verify: {{.VerificationLink}}",
            "template_engine": "go_template",
            "variables": {
                "user_name": "Test User",
                "verification_link": "https://example.com/verify?token=abc123",
                "app_name": "Test App"
            }
        }'

    print_section "2. Password Reset (go_template)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview password_reset template" \
        '{
            "subject": "Reset Your Password",
            "body_html": "<h2>Password Reset</h2><p>Hi {{.UserName}}, <a href=\"{{.ResetLink}}\">reset your password</a>. Expires in {{.ExpirationMinutes}} min.</p>",
            "body_text": "Hi {{.UserName}}, reset: {{.ResetLink}} ({{.ExpirationMinutes}} min)",
            "template_engine": "go_template",
            "variables": {
                "user_name": "Test User",
                "reset_link": "https://example.com/reset?token=xyz789",
                "expiration_minutes": "60",
                "app_name": "Test App"
            }
        }'

    print_section "3. 2FA Code (go_template)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview two_fa_code template" \
        '{
            "subject": "Your Verification Code",
            "body_html": "<h2>Code: {{.Code}}</h2><p>Expires in {{.ExpirationMinutes}} min.</p>",
            "body_text": "Code: {{.Code}} ({{.ExpirationMinutes}} min)",
            "template_engine": "go_template",
            "variables": {
                "code": "847293",
                "expiration_minutes": "5",
                "app_name": "Test App"
            }
        }'

    print_section "4. Welcome (placeholder)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview welcome template (placeholder engine)" \
        '{
            "subject": "Welcome to {app_name}",
            "body_html": "<h2>Welcome {user_name}!</h2><p>You are now part of {app_name}.</p>",
            "body_text": "Welcome {user_name}! You are now part of {app_name}.",
            "template_engine": "placeholder",
            "variables": {
                "user_name": "Test User",
                "app_name": "Test App"
            }
        }'

    print_section "5. Account Deactivated (raw_html)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview account_deactivated template (raw_html engine)" \
        '{
            "subject": "Account Deactivated",
            "body_html": "<h2 style=\"color:red;\">Account Deactivated</h2><p>Your account on {{.AppName}} has been deactivated.</p>",
            "body_text": "Your account on {{.AppName}} has been deactivated.",
            "template_engine": "raw_html",
            "variables": {
                "app_name": "Test App"
            }
        }'

    print_section "6. Password Changed (go_template)"
    do_request POST \
        "$BASE_URL/admin/email-templates/preview" \
        "Preview password_changed template" \
        '{
            "subject": "Your Password Has Been Changed",
            "body_html": "<h2>Password Changed</h2><p>Changed on {{.ChangeTime}} for {{.AppName}}.</p>",
            "body_text": "Password changed on {{.ChangeTime}} for {{.AppName}}.",
            "template_engine": "go_template",
            "variables": {
                "change_time": "2026-02-25 14:30:00 UTC",
                "app_name": "Test App"
            }
        }'
}

test_smtp_connectivity() {
    print_header "SMTP Connectivity Test"

    print_section "Test App Default SMTP"
    do_request POST \
        "$BASE_URL/admin/apps/$APP_ID/email-test" \
        "Test app default SMTP config" \
        '{
            "to_email": "'"$TEST_EMAIL"'"
        }'

    if [[ -n "$CONFIG_ID" ]]; then
        print_section "Test Specific SMTP Config"
        do_request POST \
            "$BASE_URL/admin/email-servers/$CONFIG_ID/test" \
            "Test SMTP config $CONFIG_ID" \
            '{
                "to_email": "'"$TEST_EMAIL"'"
            }'
    else
        echo -e "${YELLOW}  Skipping specific config test (CONFIG_ID not set)${NC}"
        echo ""
    fi
}

# ---------------------------------------------------------------------------
# App API Key tests
# ---------------------------------------------------------------------------
# do_request_custom makes a request with arbitrary headers.
# Args: METHOD URL DESCRIPTION EXPECTED_CODE HEADERS_ARRAY DATA
do_request_custom() {
    local method="$1"
    local url="$2"
    local description="$3"
    local expected_code="$4"
    shift 4
    local data=""
    local -a headers=()

    # Remaining args are -H "Header: Value" pairs, last non -H arg is data
    while [[ $# -gt 0 ]]; do
        if [[ "$1" == "-H" ]]; then
            headers+=(-H "$2")
            shift 2
        elif [[ "$1" == "-d" ]]; then
            data="$2"
            shift 2
        else
            shift
        fi
    done

    print_request "$method $url"

    local response
    local http_code

    local curl_args=(-s -w "\n%{http_code}" -X "$method")
    curl_args+=("${headers[@]}")
    curl_args+=(-H "Content-Type: application/json")
    if [[ -n "$data" ]]; then
        curl_args+=(-d "$data")
    fi
    curl_args+=("$url")

    response=$(curl "${curl_args[@]}" 2>&1)
    http_code=$(echo "$response" | tail -1)
    local body
    body=$(echo "$response" | sed '$d')

    check_response_expect "$description" "$expected_code" "$http_code" "$body"

    if [[ "$method" == "GET" && "$http_code" -ge 200 && "$http_code" -lt 300 ]]; then
        echo -e "  ${CYAN}Response (first 200 chars):${NC} ${body:0:200}"
    fi
    echo ""
}

test_app_api_key() {
    print_header "App API Key Restriction Tests"

    echo -e "${BOLD}  Testing that app API keys work on /app routes and are${NC}"
    echo -e "${BOLD}  rejected on /admin routes, and vice versa.${NC}"
    echo ""
    echo "  App ID:      $APP_ID"
    echo "  App API Key: ${APP_API_KEY:0:12}..."
    echo ""

    # -----------------------------------------------------------------------
    # Positive tests: App key on /app routes (should succeed)
    # -----------------------------------------------------------------------
    print_section "Positive: App key on /app routes (expect 200)"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-config" \
        "GET /app/:id/email-config with app key" \
        200 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $APP_API_KEY"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-servers" \
        "GET /app/:id/email-servers with app key" \
        200 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $APP_API_KEY"

    do_request_custom POST \
        "$BASE_URL/app/$APP_ID/email-test" \
        "POST /app/:id/email-test with app key" \
        200 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $APP_API_KEY" \
        -d '{"to_email": "'"$TEST_EMAIL"'"}'

    do_request_custom POST \
        "$BASE_URL/app/$APP_ID/send-email" \
        "POST /app/:id/send-email with app key" \
        200 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $APP_API_KEY" \
        -d '{
            "type_code": "welcome",
            "to_email": "'"$TEST_EMAIL"'",
            "variables": {
                "user_name": "App Key Test User",
                "app_name": "App Key Test"
            }
        }'

    # -----------------------------------------------------------------------
    # Negative: App key on /admin routes (should be rejected)
    # -----------------------------------------------------------------------
    print_section "Negative: App key on /admin routes (expect 401)"

    do_request_custom GET \
        "$BASE_URL/admin/email-types" \
        "App key rejected on GET /admin/email-types" \
        401 \
        -H "X-Admin-API-Key: $APP_API_KEY"

    do_request_custom GET \
        "$BASE_URL/admin/apps/$APP_ID/email-config" \
        "App key rejected on GET /admin/apps/:id/email-config" \
        401 \
        -H "X-Admin-API-Key: $APP_API_KEY"

    do_request_custom POST \
        "$BASE_URL/admin/apps/$APP_ID/email-test" \
        "App key rejected on POST /admin/apps/:id/email-test" \
        401 \
        -H "X-Admin-API-Key: $APP_API_KEY" \
        -d '{"to_email": "'"$TEST_EMAIL"'"}'

    # -----------------------------------------------------------------------
    # Negative: Admin key on /app routes (should be rejected)
    # -----------------------------------------------------------------------
    print_section "Negative: Admin key on /app routes (expect 401)"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-config" \
        "Admin key rejected on GET /app/:id/email-config" \
        401 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $ADMIN_API_KEY"

    do_request_custom POST \
        "$BASE_URL/app/$APP_ID/email-test" \
        "Admin key rejected on POST /app/:id/email-test" \
        401 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $ADMIN_API_KEY" \
        -d '{"to_email": "'"$TEST_EMAIL"'"}'

    # -----------------------------------------------------------------------
    # Negative: Missing headers on /app routes
    # -----------------------------------------------------------------------
    print_section "Negative: Missing headers on /app routes"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-config" \
        "Missing X-App-API-Key header (expect 401)" \
        401 \
        -H "X-App-ID: $APP_ID"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-config" \
        "Missing X-App-ID header (expect 400)" \
        400 \
        -H "X-App-API-Key: $APP_API_KEY"

    do_request_custom GET \
        "$BASE_URL/app/$APP_ID/email-config" \
        "Missing both headers (expect 400)" \
        400

    # -----------------------------------------------------------------------
    # Negative: Wrong App ID in URL (key bound to APP_ID but URL has different UUID)
    # -----------------------------------------------------------------------
    print_section "Negative: Wrong App ID in URL (expect 403)"

    local WRONG_APP_ID="99999999-9999-9999-9999-999999999999"
    do_request_custom GET \
        "$BASE_URL/app/$WRONG_APP_ID/email-config" \
        "URL app ID doesn't match X-App-ID header (expect 403)" \
        403 \
        -H "X-App-ID: $APP_ID" \
        -H "X-App-API-Key: $APP_API_KEY"
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

echo -e "${BOLD}${BLUE}"
echo "  ================================================================"
echo "  Auth API - Email Testing Script"
echo "  ================================================================"
echo -e "${NC}"
echo "  Base URL:      $BASE_URL"
echo "  App ID:        $APP_ID"
echo "  Test Email:    $TEST_EMAIL"
echo "  Config ID:     ${CONFIG_ID:-<not set>}"
echo "  App API Key:   ${APP_API_KEY:0:12}..."
echo ""

case "${1:-}" in
    --setup)
        setup_smtp
        send_all_email_types
        ;;
    --preview-only)
        preview_all_templates
        ;;
    --send-only)
        send_all_email_types
        ;;
    --smtp-test)
        test_smtp_connectivity
        ;;
    --app-key)
        test_app_api_key
        ;;
    --list)
        list_resources
        ;;
    --all)
        setup_smtp
        list_resources
        send_all_email_types
        preview_all_templates
        test_smtp_connectivity
        test_app_api_key
        ;;
    --help|-h)
        echo "Usage: $0 [OPTION]"
        echo ""
        echo "Options:"
        echo "  (no args)       Send all 6 email types + preview all templates"
        echo "  --setup         Configure SMTP first, then send all emails"
        echo "  --preview-only  Only run template preview tests"
        echo "  --send-only     Only send emails (skip previews)"
        echo "  --smtp-test     Only test SMTP connectivity"
        echo "  --app-key       Test app API key restrictions and access"
        echo "  --list          List email types, templates, variables"
        echo "  --all           Run everything (setup + list + send + preview + SMTP + app-key)"
        echo "  --help          Show this help"
        echo ""
        echo "Environment variables:"
        echo "  BASE_URL        API base URL         (default: http://localhost:8080)"
        echo "  ADMIN_API_KEY   Admin API key         (required)"
        echo "  APP_API_KEY     App API key           (required for --app-key)"
        echo "  APP_ID          Application UUID      (required)"
        echo "  CONFIG_ID       SMTP config UUID      (optional, for --smtp-test)"
        echo "  TEST_EMAIL      Target email address   (default: test@example.com)"
        echo ""
        echo "  SMTP_HOST       SMTP server hostname  (default: smtp.gmail.com)"
        echo "  SMTP_PORT       SMTP server port      (default: 587)"
        echo "  SMTP_USERNAME   SMTP username         (for --setup)"
        echo "  SMTP_PASSWORD   SMTP password         (for --setup)"
        echo "  SMTP_FROM_ADDRESS  Sender email       (for --setup)"
        echo "  SMTP_FROM_NAME     Sender name        (for --setup)"
        echo ""
        echo "Examples:"
        echo "  # Quick test with dev mode (emails logged to server console):"
        echo "  ADMIN_API_KEY=mykey APP_ID=abc-123 $0"
        echo ""
        echo "  # Full test with Gmail SMTP:"
        echo "  ADMIN_API_KEY=mykey APP_ID=abc-123 TEST_EMAIL=me@gmail.com \\"
        echo "    SMTP_USERNAME=me@gmail.com SMTP_PASSWORD=app-password \\"
        echo "    SMTP_FROM_ADDRESS=me@gmail.com $0 --setup"
        echo ""
        echo "  # Test app API key restrictions:"
        echo "  ADMIN_API_KEY=ak_xxx APP_API_KEY=apk_yyy APP_ID=abc-123 $0 --app-key"
        exit 0
        ;;
    "")
        send_all_email_types
        preview_all_templates
        ;;
    *)
        echo -e "${RED}Unknown option: $1${NC}"
        echo "Run '$0 --help' for usage information."
        exit 1
        ;;
esac

print_summary
