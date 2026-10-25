import { useEffect, useState } from "react";
import { api } from "../lib/api";

export default function QueueMetrics() {
  const [status, setStatus] = useState<{ active: number; queued: number } | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setStatus(await api.workerStatus());
    } catch (e: any) {
      setError(e.message);
    }
  }

  useEffect(() => {
    load();
    const t = setInterval(load, 3000);
    return () => clearInterval(t);
  }, []);

  return (
    <div className="panel">
      <h2>Queue / Retry Metrics</h2>
      {error && <div className="error-banner">{error}</div>}
      {status && (
        <table>
          <tbody>
            <tr>
              <th>Active workers</th>
              <td>{status.active}</td>
            </tr>
            <tr>
              <th>Queued jobs</th>
              <td>{status.queued}</td>
            </tr>
          </tbody>
        </table>
      )}
      <p className="hint">
        See <code>GET /metrics</code> for the full counter/gauge set, including
        dependency call outcomes broken down by retry layer.
      </p>
    </div>
  );
}