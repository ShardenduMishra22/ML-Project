from pathlib import Path

from app.model import XGBRiskModel
from app.train_model import train_model


def test_model_inference(tmp_path: Path) -> None:
    model_path = tmp_path / "xgb_model.json"
    train_model(model_path)

    model = XGBRiskModel(str(model_path))
    model.load()

    prediction, probability = model.predict([
        120.0,
        0.55,
        0.12,
        0.21,
        1.1,
        3.0,
        127.0,
        0.05,
        1800.0,
        0.3,
    ])

    assert prediction in {0, 1, 2}
    assert 0.0 <= probability <= 1.0
