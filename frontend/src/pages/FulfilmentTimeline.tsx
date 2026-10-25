import { useState } from "react";
import { api, AuditEvent } from "../lib/api";

export default function FulfilmentTimeline() {
  const [tenant, setTenant] = useState("tenant-a");
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setEvents(await api.listAudit(tenant));
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="panel">
      <h2>Fulfilment Timeline (Audit Trail)</h2>
      <label>
        Tenant: <input value={tenant} onChange={(e) => setTenant(e.target.value)} />
      </label>{" "}
      <button className="action" onClick={load}>
        Load
      </button>
      {error && <div className="error-banner">{error}</div>}
      <table>
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Action</th>
            <th>Order</th>
            <th>Actor</th>
          </tr>
        </thead>
        <tbody>
          {events
            .slice()
            .sort((a, b) => a.Timestamp.localeCompare(b.Timestamp))
            .map((e) => (
              <tr key={e.ID}>
                <td>{e.Timestamp}</td>
                <td>{e.Action}</td>
                <td>{e.EntityID}</td>
                <td>{e.ActorID}</td>
              </tr>
            ))}
        </tbody>
      </table>
      {events.length === 0 && (
        <p className="hint">
          No audit events loaded yet. Approve, cancel or fulfil an order from the Orders tab, then reload here.
        </p>
      )}
    </div>
  );
}