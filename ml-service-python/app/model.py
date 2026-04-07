from pathlib import Path

import numpy as np
from xgboost import XGBClassifier


class XGBRiskModel:
    def __init__(self, model_path: str) -> None:
        self.model_path = Path(model_path)
        self.model = XGBClassifier()

    def load(self) -> None:
        self.model.load_model(str(self.model_path))

    def predict(self, features: list[float]) -> tuple[int, float]:
        if len(features) != 10:
            raise ValueError("expected 10 features")

        x = np.array(features, dtype=np.float32).reshape(1, -1)
        proba = self.model.predict_proba(x)[0]
        pred = int(np.argmax(proba))
        confidence = float(proba[pred])
        return pred, confidence
