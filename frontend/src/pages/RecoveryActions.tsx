import { useState } from "react";
import { api } from "../lib/api";

export default function RecoveryActions() {
  const [orderIds, setOrderIds] = useState("ord-1");
  const [status, setStatus] = useState("cancelled");
  const [actor, setActor] = useState("support-agent");
  const [result, setResult] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function apply() {
    setError(null);
    setResult(null);
    try {
      const ids = orderIds.split(",").map((s) => s.trim()).filter(Boolean);
      const res = await api.adminBulkStatus(ids, status, actor);
      setResult(`Updated ${res.updated} order(s).`);
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="panel">
      <h2>Recovery Actions</h2>
      <p className="hint">
        Administrative bulk status override for incident recovery. Use sparingly —
        this bypasses the ordinary order workflow.
      </p>
      <div>
        <label>
          Order IDs (comma-separated): <input value={orderIds} onChange={(e) => setOrderIds(e.target.value)} style={{ width: "20rem" }} />
        </label>
      </div>
      <div style={{ marginTop: "0.5rem" }}>
        <label>
          New status:{" "}
          <select value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="pending">pending</option>
            <option value="approved">approved</option>
            <option value="reserved">reserved</option>
            <option value="fulfilling">fulfilling</option>
            <option value="fulfilled">fulfilled</option>
            <option value="cancelled">cancelled</option>
            <option value="failed">failed</option>
          </select>
        </label>{" "}
        <label>
          Actor: <input value={actor} onChange={(e) => setActor(e.target.value)} />
        </label>
      </div>
      <div style={{ marginTop: "0.75rem" }}>
        <button className="action danger" onClick={apply}>
          Apply Bulk Status Override
        </button>
      </div>
      {result && <p className="hint">{result}</p>}
      {error && <div className="error-banner">{error}</div>}
    </div>
  );
}