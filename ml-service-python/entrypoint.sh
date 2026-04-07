#!/usr/bin/env sh
set -e

mkdir -p /app/models
if [ ! -f /app/models/xgb_model.json ]; then
  cp /app/bootstrap-model/xgb_model.json /app/models/xgb_model.json
fi

exec uvicorn app.main:app --host 0.0.0.0 --port 8000
