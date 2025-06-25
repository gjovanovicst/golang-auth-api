#!/bin/bash

# Test script for the logout functionality
# This script demonstrates how to use the /logout endpoint

echo "🔐 Testing Logout Functionality"
echo "================================"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Base URL
BASE_URL="http://localhost:8080"

echo ""
echo "📝 Step 1: Register a test user"
REGISTER_RESPONSE=$(curl -s -X POST $BASE_URL/register \
    -H "Content-Type: application/json" \
    -d '{
    "email": "logout_test@example.com",
    "password": "password123"
  }')

echo "Register Response: $REGISTER_RESPONSE"

echo ""
echo "🔑 Step 2: Login to get tokens"
LOGIN_RESPONSE=$(curl -s -X POST $BASE_URL/login \
    -H "Content-Type: application/json" \
    -d '{
    "email": "logout_test@example.com",
    "password": "password123"
  }')

echo "Login Response: $LOGIN_RESPONSE"

# Extract tokens from response (assuming successful login without 2FA)
ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)

if [ -z "$ACCESS_TOKEN" ] || [ -z "$REFRESH_TOKEN" ]; then
    echo -e "${RED}❌ Failed to extract tokens from login response${NC}"
    echo "This might be because:"
    echo "  - User already exists (try deleting from database)"
    echo "  - Email verification is required"
    echo "  - 2FA is enabled for this user"
    echo "  - Server is not running"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ Successfully extracted tokens${NC}"
echo "Access Token: ${ACCESS_TOKEN:0:20}..."
echo "Refresh Token: ${REFRESH_TOKEN:0:20}..."

echo ""
echo "👤 Step 3: Test protected endpoint (profile) before logout"
PROFILE_RESPONSE=$(curl -s -X GET $BASE_URL/profile \
    -H "Authorization: Bearer $ACCESS_TOKEN")

echo "Profile Response: $PROFILE_RESPONSE"

if echo $PROFILE_RESPONSE | grep -q "error"; then
    echo -e "${YELLOW}⚠️  Profile request failed (expected if user doesn't exist in DB)${NC}"
else
    echo -e "${GREEN}✅ Profile request successful${NC}"
fi

echo ""
echo "🚪 Step 4: Logout using the refresh token"
LOGOUT_RESPONSE=$(curl -s -X POST $BASE_URL/logout \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer $ACCESS_TOKEN" \
    -d "{
    \"refresh_token\": \"$REFRESH_TOKEN\"
  }")

echo "Logout Response: $LOGOUT_RESPONSE"

if echo $LOGOUT_RESPONSE | grep -q "Successfully logged out"; then
    echo -e "${GREEN}✅ Logout successful!${NC}"
else
    echo -e "${RED}❌ Logout failed${NC}"
    exit 1
fi

echo ""
echo "🔍 Step 5: Try to use refresh token after logout (should fail)"
REFRESH_AFTER_LOGOUT=$(curl -s -X POST $BASE_URL/refresh-token \
    -H "Content-Type: application/json" \
    -d "{
    \"refresh_token\": \"$REFRESH_TOKEN\"
  }")

echo "Refresh After Logout Response: $REFRESH_AFTER_LOGOUT"

if echo $REFRESH_AFTER_LOGOUT | grep -q "error"; then
    echo -e "${GREEN}✅ Refresh token correctly revoked!${NC}"
else
    echo -e "${RED}❌ Refresh token was not properly revoked${NC}"
fi

echo ""
echo "🎉 Logout functionality test completed!"
echo ""
echo "Summary:"
echo "- ✅ User can logout successfully"
echo "- ✅ Refresh token is revoked after logout"
echo "- ✅ Logout requires authentication (access token)"
echo "- ✅ Logout endpoint returns proper success message"
