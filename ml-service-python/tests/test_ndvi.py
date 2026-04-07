import numpy as np

from app.satellite import compute_ndvi


def test_compute_ndvi_formula() -> None:
    nir = np.array([0.8, 0.4, 0.2], dtype=np.float32)
    red = np.array([0.2, 0.2, 0.2], dtype=np.float32)

    ndvi = compute_ndvi(nir, red)
    expected = (nir - red) / (nir + red + 1e-6)

    assert np.allclose(ndvi, expected)
