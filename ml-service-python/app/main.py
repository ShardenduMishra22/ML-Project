from __future__ import annotations

import os
from pathlib import Path

from fastapi import FastAPI, HTTPException

from app.model import XGBRiskModel
from app.satellite import SatelliteFeatureService
from app.schemas import (
    PredictRequest,
    PredictResponse,
    SatelliteFeaturesRequest,
    SatelliteFeaturesResponse,
)
from app.train_model import train_model

MODEL_PATH = os.getenv("MODEL_PATH", "/app/models/xgb_model.json")

app = FastAPI(title="Land Risk ML Service", version="1.0.0")
model = XGBRiskModel(MODEL_PATH)
satellite_service = SatelliteFeatureService()


@app.on_event("startup")
def startup() -> None:
    model_path = Path(MODEL_PATH)
    if not model_path.exists():
        model_path.parent.mkdir(parents=True, exist_ok=True)
        train_model(model_path)
    model.load()


@app.get("/health")
def health() -> dict[str, str]:
    return {"status": "ok"}


@app.post("/predict", response_model=PredictResponse)
def predict(request: PredictRequest) -> PredictResponse:
    try:
        prediction, probability = model.predict(request.features)
    except Exception as exc:
        raise HTTPException(status_code=400, detail=str(exc)) from exc
    return PredictResponse(prediction=prediction, probability=probability)


@app.post("/satellite-features", response_model=SatelliteFeaturesResponse)
def satellite_features(request: SatelliteFeaturesRequest) -> SatelliteFeaturesResponse:
    try:
        features = satellite_service.compute_features(request)
    except Exception as exc:
        raise HTTPException(status_code=503, detail=str(exc)) from exc
    return SatelliteFeaturesResponse(**features)
