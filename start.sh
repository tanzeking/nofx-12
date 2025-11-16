#!/bin/sh
set -e

# Auto-create config.json if not exists
if [ ! -f /app/config.json ]; then
  echo "Creating config.json from example..."
  cp /app/config.json.example /app/config.json
fi

# Ensure nofx is executable
chmod +x /app/nofx

# Start backend service (background)
echo "Starting backend service..."
/app/nofx &
BACKEND_PID=$!

# Wait for backend to start
sleep 2

# Start Nginx (foreground, keep container alive)
echo "Starting Nginx..."
nginx -g "daemon off;" &
NGINX_PID=$!

echo "All services started (Backend PID: $BACKEND_PID, Nginx PID: $NGINX_PID)"

# Wait for any process to exit
wait $BACKEND_PID $NGINX_PID
