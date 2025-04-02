#!/bin/sh

# Set default values if environment variables are not set
PANEL_PORT=${PANEL_PORT:-2095}
PANEL_PATH=${PANEL_PATH:-/app}
SUB_PORT=${SUB_PORT:-2096}
SUB_PATH=${SUB_PATH:-/sub}

# Apply settings
/app/sui setting \
  -port ${PANEL_PORT} \
  -path ${PANEL_PATH} \
  -subPort ${SUB_PORT} \
  -subPath ${SUB_PATH}

# Start the service
exec /app/sui