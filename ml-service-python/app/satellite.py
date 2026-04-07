from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timedelta, timezone
from typing import Any

import numpy as np
import planetary_computer
import rasterio
from pyproj import Transformer
from pystac_client import Client
from rasterio.windows import Window
from shapely.geometry import shape

from app.schemas import SatelliteFeaturesRequest

STAC_URL = "https://planetarycomputer.microsoft.com/api/stac/v1"


def compute_ndvi(nir: np.ndarray, red: np.ndarray) -> np.ndarray:
    return (nir - red) / (nir + red + 1e-6)


class SatelliteFeatureService:
    def __init__(self) -> None:
        self.catalog = Client.open(STAC_URL, modifier=planetary_computer.sign_inplace)

    def compute_features(self, req: SatelliteFeaturesRequest) -> dict[str, float]:
        lon, lat = self._resolve_point(req)
        end = datetime.now(timezone.utc)
        start = end - timedelta(days=180)
        date_range = f"{start.date().isoformat()}/{end.date().isoformat()}"

        jobs = [
            ("sentinel-2-l2a", ["B04", "red"], ["B08", "nir", "nir08"]),
            ("landsat-c2-l2", ["red", "SR_B4"], ["nir08", "nir", "SR_B5"]),
        ]

        measurements: list[tuple[float, float, float]] = []
        with ThreadPoolExecutor(max_workers=2) as pool:
            futures = {
                pool.submit(self._compute_collection_features, collection, lon, lat, date_range, red_keys, nir_keys): collection
                for collection, red_keys, nir_keys in jobs
            }
            for future in as_completed(futures):
                try:
                    measurement = future.result()
                except Exception:
                    continue
                if measurement is not None:
                    measurements.append(measurement)

        if not measurements:
            raise RuntimeError("unable to derive satellite features from Sentinel-2/Landsat")

        ndvi = float(np.mean([m[0] for m in measurements]))
        vegetation_density = float(np.mean([m[1] for m in measurements]))
        water_overlap_ratio = float(np.mean([m[2] for m in measurements]))

        return {
            "ndvi": ndvi,
            "vegetation_density": vegetation_density,
            "water_overlap_ratio": water_overlap_ratio,
        }

    def _compute_collection_features(
        self,
        collection: str,
        lon: float,
        lat: float,
        date_range: str,
        red_keys: list[str],
        nir_keys: list[str],
    ) -> tuple[float, float, float] | None:
        search = self.catalog.search(
            collections=[collection],
            intersects={"type": "Point", "coordinates": [lon, lat]},
            datetime=date_range,
            query={"eo:cloud_cover": {"lt": 40}},
            limit=3,
        )
        items = list(search.items())
        if not items:
            return None

        for item in items:
            red_href = self._find_asset_href(item.assets, red_keys)
            nir_href = self._find_asset_href(item.assets, nir_keys)
            if not red_href or not nir_href:
                continue
            try:
                red = self._read_band_window(red_href, lon, lat)
                nir = self._read_band_window(nir_href, lon, lat)
            except Exception:
                continue

            if red.size == 0 or nir.size == 0:
                continue
            h = min(red.shape[0], nir.shape[0])
            w = min(red.shape[1], nir.shape[1])
            red = red[:h, :w]
            nir = nir[:h, :w]

            ndvi_arr = compute_ndvi(nir, red)
            finite = np.isfinite(ndvi_arr)
            if not np.any(finite):
                continue
            ndvi_clean = ndvi_arr[finite]
            ndvi = float(np.mean(ndvi_clean))
            vegetation_density = float(np.mean(ndvi_clean > 0.35))
            water_overlap_ratio = float(np.mean(ndvi_clean < 0.05))
            return ndvi, vegetation_density, water_overlap_ratio

        return None

    @staticmethod
    def _find_asset_href(assets: dict[str, Any], candidates: list[str]) -> str | None:
        lower_lookup = {k.lower(): k for k in assets.keys()}
        for key in candidates:
            if key.lower() in lower_lookup:
                return assets[lower_lookup[key.lower()]].href
        return None

    @staticmethod
    def _read_band_window(href: str, lon: float, lat: float, window_size: int = 64) -> np.ndarray:
        with rasterio.open(href) as ds:
            x, y = lon, lat
            if ds.crs and str(ds.crs).upper() != "EPSG:4326":
                transformer = Transformer.from_crs("EPSG:4326", ds.crs, always_xy=True)
                x, y = transformer.transform(lon, lat)

            row, col = ds.index(x, y)
            row = int(np.clip(row, 0, ds.height - 1))
            col = int(np.clip(col, 0, ds.width - 1))

            half = window_size // 2
            row_off = max(0, row - half)
            col_off = max(0, col - half)
            height = min(window_size, ds.height - row_off)
            width = min(window_size, ds.width - col_off)
            window = Window(col_off=col_off, row_off=row_off, width=width, height=height)

            arr = ds.read(1, window=window, masked=True).astype(np.float32)
            return arr.filled(np.nan)

    @staticmethod
    def _resolve_point(req: SatelliteFeaturesRequest) -> tuple[float, float]:
        if req.latitude is not None and req.longitude is not None:
            return float(req.longitude), float(req.latitude)
        if req.geometry is not None:
            geom = shape(req.geometry)
            centroid = geom.centroid
            return float(centroid.x), float(centroid.y)
        raise ValueError("location could not be resolved")
