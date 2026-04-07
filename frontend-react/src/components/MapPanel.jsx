import { MapContainer, Marker, Polygon, TileLayer, Tooltip } from "react-leaflet";
import L from "leaflet";

const markerIcon = L.icon({
  iconUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-icon.png",
  shadowUrl: "https://unpkg.com/leaflet@1.9.4/dist/images/marker-shadow.png",
  iconSize: [25, 41],
  iconAnchor: [12, 41],
});

function riskColor(riskClass) {
  if (riskClass === "High") return "#c13b1f";
  if (riskClass === "Medium") return "#df8a2d";
  return "#2e7d4f";
}

function geoJsonToLeafletPolygon(geometry) {
  if (!geometry || !geometry.type || !geometry.coordinates) return null;

  if (geometry.type === "Polygon") {
    return geometry.coordinates[0].map(([lng, lat]) => [lat, lng]);
  }
  if (geometry.type === "MultiPolygon") {
    return geometry.coordinates[0][0].map(([lng, lat]) => [lat, lng]);
  }
  return null;
}

export default function MapPanel({ report, input }) {
  const lat = input?.latitude ?? 12.9716;
  const lng = input?.longitude ?? 77.5946;
  const polygon = geoJsonToLeafletPolygon(report?.geometry);
  const color = riskColor(report?.riskClass);

  return (
    <div className="card map-card">
      <h2>Spatial Risk View</h2>
      <MapContainer center={[lat, lng]} zoom={13} className="map">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />

        <Marker position={[lat, lng]} icon={markerIcon}>
          <Tooltip direction="top" offset={[0, -20]} opacity={1}>
            Input location
          </Tooltip>
        </Marker>

        {polygon && (
          <Polygon positions={polygon} pathOptions={{ color, fillColor: color, fillOpacity: 0.35 }}>
            <Tooltip sticky>{report?.riskClass || "Risk"} risk overlay</Tooltip>
          </Polygon>
        )}
      </MapContainer>
    </div>
  );
}
