from typing import Any

from pydantic import BaseModel, Field, model_validator


class PredictRequest(BaseModel):
    features: list[float] = Field(..., min_length=10, max_length=10)


class PredictResponse(BaseModel):
    prediction: int
    probability: float


class SatelliteFeaturesRequest(BaseModel):
    latitude: float | None = None
    longitude: float | None = None
    geometry: dict[str, Any] | None = None

    @model_validator(mode="after")
    def validate_location_input(self) -> "SatelliteFeaturesRequest":
        if self.latitude is not None and self.longitude is not None:
            return self
        if self.geometry is not None:
            return self
        raise ValueError("Provide either latitude+longitude or geometry")


class SatelliteFeaturesResponse(BaseModel):
    ndvi: float
    vegetation_density: float
    water_overlap_ratio: float
