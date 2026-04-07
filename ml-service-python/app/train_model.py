import argparse
from pathlib import Path

import numpy as np
from sklearn.model_selection import train_test_split
from xgboost import XGBClassifier


def build_training_data(n_samples: int = 12000, seed: int = 42) -> tuple[np.ndarray, np.ndarray]:
    rng = np.random.default_rng(seed)

    distance_to_water = rng.uniform(5, 3000, n_samples)
    water_overlap_ratio = rng.uniform(0, 1, n_samples)
    ndvi = rng.uniform(-0.2, 0.9, n_samples)
    vegetation_density = rng.uniform(0, 1, n_samples)
    forest_proximity = rng.uniform(0, 2, n_samples)
    jurisdiction_flags = rng.uniform(0, 6, n_samples)
    zonation_code = rng.uniform(1, 1000, n_samples)
    nearby_assets_density = rng.uniform(0, 0.15, n_samples)
    survey_area = rng.uniform(200, 120000, n_samples)
    anomaly_score = rng.uniform(0, 1, n_samples)

    X = np.column_stack(
        [
            distance_to_water,
            water_overlap_ratio,
            ndvi,
            vegetation_density,
            forest_proximity,
            jurisdiction_flags,
            zonation_code,
            nearby_assets_density,
            survey_area,
            anomaly_score,
        ]
    )

    risk_signal = (
        1.8 * water_overlap_ratio
        + 0.9 * anomaly_score
        + 0.6 * (distance_to_water < 300)
        + 0.4 * jurisdiction_flags / 6.0
        + 0.3 * nearby_assets_density * 10
        - 0.5 * ndvi
        - 0.3 * vegetation_density
    )

    y = np.where(risk_signal > 1.4, 2, np.where(risk_signal > 0.8, 1, 0))
    return X, y


def train_model(output_path: Path) -> None:
    X, y = build_training_data()
    X_train, X_test, y_train, y_test = train_test_split(X, y, test_size=0.2, random_state=7, stratify=y)

    model = XGBClassifier(
        objective="multi:softprob",
        num_class=3,
        n_estimators=250,
        max_depth=5,
        learning_rate=0.05,
        subsample=0.9,
        colsample_bytree=0.9,
        reg_lambda=1.0,
        random_state=42,
        eval_metric="mlogloss",
    )
    model.fit(X_train, y_train)

    acc = float((model.predict(X_test) == y_test).mean())
    output_path.parent.mkdir(parents=True, exist_ok=True)
    model.save_model(str(output_path))
    print(f"model_saved={output_path} accuracy={acc:.4f}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", type=Path, default=Path("/app/models/xgb_model.json"))
    args = parser.parse_args()
    train_model(args.output)


if __name__ == "__main__":
    main()
