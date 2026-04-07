import { useState } from "react";

const defaultState = {
  latitude: "",
  longitude: "",
  surveyNumber: "",
  villageId: "",
  pincode: "",
  surveyType: "parcel",
};

export default function InputForm({ onSubmit, loading }) {
  const [form, setForm] = useState(defaultState);

  function setValue(key, value) {
    setForm((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(event) {
    event.preventDefault();

    const payload = {
      surveyNumber: form.surveyNumber || undefined,
      villageId: form.villageId || undefined,
      pincode: form.pincode || undefined,
      surveyType: form.surveyType || undefined,
      latitude: form.latitude !== "" ? Number(form.latitude) : undefined,
      longitude: form.longitude !== "" ? Number(form.longitude) : undefined,
    };

    onSubmit(payload);
  }

  return (
    <form className="card input-form" onSubmit={handleSubmit}>
      <h2>Land Risk Input</h2>
      <p className="muted">Provide either lat/lon or survey + village inputs.</p>

      <div className="grid two">
        <label>
          Latitude
          <input
            type="number"
            step="any"
            value={form.latitude}
            onChange={(e) => setValue("latitude", e.target.value)}
            placeholder="12.9716"
          />
        </label>
        <label>
          Longitude
          <input
            type="number"
            step="any"
            value={form.longitude}
            onChange={(e) => setValue("longitude", e.target.value)}
            placeholder="77.5946"
          />
        </label>
      </div>

      <div className="grid three">
        <label>
          Survey Number
          <input
            type="text"
            value={form.surveyNumber}
            onChange={(e) => setValue("surveyNumber", e.target.value)}
            placeholder="12/3"
          />
        </label>
        <label>
          Village ID
          <input
            type="text"
            value={form.villageId}
            onChange={(e) => setValue("villageId", e.target.value)}
            placeholder="301001"
          />
        </label>
        <label>
          Pincode
          <input
            type="text"
            value={form.pincode}
            onChange={(e) => setValue("pincode", e.target.value)}
            placeholder="560001"
          />
        </label>
      </div>

      <label>
        Survey Type
        <select value={form.surveyType} onChange={(e) => setValue("surveyType", e.target.value)}>
          <option value="parcel">Parcel</option>
          <option value="plot">Plot</option>
          <option value="land">Land</option>
        </select>
      </label>

      <button type="submit" disabled={loading}>
        {loading ? "Analyzing..." : "Run Risk Assessment"}
      </button>
    </form>
  );
}
