#!/bin/bash

# Simple REST API Test
set -e

PORT="${SERVER_PORT:-8080}"
BASE_URL="http://localhost:${PORT}/api/v1"

echo "🧪 Simple REST API Test"
echo "Testing server at: $BASE_URL"

# Test 1: Health check
echo "1. Testing health check..."
http_code=$(curl -s -w "%{http_code}" -o /tmp/health.json "$BASE_URL/health")
if [ "$http_code" = "200" ]; then
    echo "✅ Health check passed"
else
    echo "❌ Health check failed (HTTP $http_code)"
    exit 1
fi

# Test 2: Login
echo "2. Testing login..."
login_data='{"username":"admin","password":"password"}'
http_code=$(curl -s -w "%{http_code}" -o /tmp/login.json \
    -X POST "$BASE_URL/login" \
    -H "Content-Type: application/json" \
    -d "$login_data")

if [ "$http_code" = "200" ]; then
    TOKEN=$(grep -o '"token":"[^"]*"' /tmp/login.json | cut -d'"' -f4)
    echo "✅ Login passed, token: ${TOKEN:0:20}..."
else
    echo "❌ Login failed (HTTP $http_code)"
    exit 1
fi

# Test 3: PUT data
echo "3. Testing PUT..."
put_data='{"value":"test123"}'
http_code=$(curl -s -w "%{http_code}" -o /tmp/put.json \
    -X PUT "$BASE_URL/kv/testkey" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "$put_data")

if [ "$http_code" = "200" ]; then
    echo "✅ PUT passed"
else
    echo "❌ PUT failed (HTTP $http_code)"
    exit 1
fi

# Test 4: GET data
echo "4. Testing GET..."
http_code=$(curl -s -w "%{http_code}" -o /tmp/get.json \
    -X GET "$BASE_URL/kv/testkey" \
    -H "Authorization: Bearer $TOKEN")

if [ "$http_code" = "200" ]; then
    if grep -q "test123" /tmp/get.json; then
        echo "✅ GET passed"
    else
        echo "❌ GET returned wrong value"
        exit 1
    fi
else
    echo "❌ GET failed (HTTP $http_code)"
    exit 1
fi

echo "🎉 All tests passed!"
rm -f /tmp/health.json /tmp/login.json /tmp/put.json /tmp/get.json