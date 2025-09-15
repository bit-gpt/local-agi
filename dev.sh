#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Load environment variables from .env file if it exists
if [ -f .env ]; then
    echo -e "${GREEN}📄 Loading environment variables from .env file...${NC}"
    # Use set -a to automatically export variables, then source the file
    set -a
    source .env
    set +a
fi

echo -e "${BLUE}🔧 Environment Setup:${NC}"
echo ""

# Function to cleanup background processes
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up processes...${NC}"
    jobs -p | xargs -r kill
    exit 0
}

# Trap Ctrl+C and cleanup
trap cleanup INT

# Get local network IP
LOCAL_IP=$(ifconfig | grep "inet " | grep -v 127.0.0.1 | awk '{print $2}' | head -n1)

echo -e "${GREEN}🎯 Starting development servers...${NC}"
echo ""
echo -e "  ➜  ${BLUE}Local:${NC}        http://localhost:3000"
if [ -n "$LOCAL_IP" ]; then
    echo -e "  ➜  ${BLUE}Network:${NC}      http://$LOCAL_IP:3000"
fi
echo ""

# Start React dev server in background
echo -e "${GREEN}Starting React UI dev server...${NC}"
cd webui/react-ui
bun run dev &
REACT_PID=$!

# Wait a moment for React server to start
sleep 2

# Go back to root directory
cd ../../

# Start Go server with air (live reload)
echo -e "${GREEN}Starting Go backend with live reload...${NC}"
air &
GO_PID=$!

# Wait for both processes
wait $REACT_PID $GO_PID
