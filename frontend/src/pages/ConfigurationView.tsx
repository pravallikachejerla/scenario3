import { useState } from "react";
import { api } from "../lib/api";

export default function ConfigurationView() {
  const [tenant, setTenant] = useState("tenant-a");
  const [key, setKey] = useState("fulfilment.dependency_timeout_ms");
  const [value, setValue] = useState("");
  const [current, setCurrent] = useState<any>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setCurrent(await api.getConfig(tenant, key));
    } catch (e: any) {
      setError(e.message);
      setCurrent(null);
    }
  }

  async function save() {
    setError(null);
    try {
      await api.putConfig(tenant, key, value);
      await load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div className="panel">
      <h2>Configuration</h2>
      <label>
        Tenant: <input value={tenant} onChange={(e) => setTenant(e.target.value)} />
      </label>{" "}
      <label>
        Key: <input value={key} onChange={(e) => setKey(e.target.value)} style={{ width: "18rem" }} />
      </label>{" "}
      <button className="action" onClick={load}>
        Load
      </button>
      {error && <div className="error-banner">{error}</div>}
      {current && (
        <p>
          Current value: <b>{current.Value}</b> (version {current.Version})
        </p>
      )}
      <div style={{ marginTop: "0.75rem" }}>
        <label>
          New value: <input value={value} onChange={(e) => setValue(e.target.value)} />
        </label>{" "}
        <button className="action" onClick={save}>
          Save
        </button>
      </div>
    </div>
  );
}