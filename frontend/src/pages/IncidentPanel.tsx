import { useEffect, useState } from "react";

interface Incident {
  id: string;
  date: string;
  severity: string;
  business_impact: string;
  observed_symptoms: string;
  workaround: string;
}

export default function IncidentPanel() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch("/docs/operational-evidence/incident-register.json")
      .then((r) => (r.ok ? r.json() : Promise.reject(new Error(String(r.status)))))
      .then((data) => setIncidents(data.incidents ?? []))
      .catch(() => setError("Incident register not reachable from this preview — see docs/operational-evidence/incident-register.json in the repository."));
  }, []);

  return (
    <div className="panel">
      <h2>Incident Panel</h2>
      {error && <p className="hint">{error}</p>}
      <table>
        <thead>
          <tr>
            <th>ID</th>
            <th>Date</th>
            <th>Severity</th>
            <th>Business impact</th>
            <th>Observed symptoms</th>
          </tr>
        </thead>
        <tbody>
          {incidents.map((i) => (
            <tr key={i.id}>
              <td>{i.id}</td>
              <td>{i.date}</td>
              <td>
                <span className="status-pill">{i.severity}</span>
              </td>
              <td>{i.business_impact}</td>
              <td>{i.observed_symptoms}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}