#!/bin/bash

echo "🔍 Testing Authentication API..."

# Test 1: API is responding
echo "1. Testing API health..."
curl -s -H "X-App-ID: 00000000-0000-0000-0000-000000000001" http://localhost:8080/register > /dev/null
if [ $? -eq 0 ]; then
    echo "✅ API is responding"
else
    echo "❌ API is not responding"
    exit 1
fi

# Test 2: Registration
echo "2. Testing user registration..."
REGISTER_RESPONSE=$(curl -s -H "X-App-ID: 00000000-0000-0000-0000-000000000001" -X POST http://localhost:8080/register \
    -H "Content-Type: application/json" \
-d '{"email": "test@example.com", "password": "password123"}')

if [[ $REGISTER_RESPONSE == *"User registered successfully"* ]]; then
    echo "✅ Registration working"
else
    echo "❌ Registration failed: $REGISTER_RESPONSE"
fi

# Test 3: Invalid login (email not verified)
echo "3. Testing login with unverified email..."
LOGIN_RESPONSE=$(curl -s -H "X-App-ID: 00000000-0000-0000-0000-000000000001" -X POST http://localhost:8080/login \
    -H "Content-Type: application/json" \
-d '{"email": "test@example.com", "password": "password123"}')

if [[ $LOGIN_RESPONSE == *"Email not verified"* ]]; then
    echo "✅ Email verification check working"
else
    echo "⚠️  Login response: $LOGIN_RESPONSE"
fi

# Test 4: Invalid credentials
echo "4. Testing invalid credentials..."
INVALID_LOGIN=$(curl -s -H "X-App-ID: 00000000-0000-0000-0000-000000000001" -X POST http://localhost:8080/login \
    -H "Content-Type: application/json" \
-d '{"email": "wrong@email.com", "password": "wrongpass"}')

if [[ $INVALID_LOGIN == *"Invalid credentials"* ]]; then
    echo "✅ Invalid credentials check working"
else
    echo "⚠️  Invalid login response: $INVALID_LOGIN"
fi

# Test 5: Protected route without token
echo "5. Testing protected route without token..."
PROTECTED_RESPONSE=$(curl -s -H "X-App-ID: 00000000-0000-0000-0000-000000000001" http://localhost:8080/profile)

if [[ $PROTECTED_RESPONSE == *"Authorization header required"* ]]; then
    echo "✅ Protected route security working"
else
    echo "⚠️  Protected route response: $PROTECTED_RESPONSE"
fi

echo "🎉 API testing completed!"