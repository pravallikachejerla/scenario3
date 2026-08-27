import { useState } from "react";
import { api, InventoryItem } from "../lib/api";

export default function InventoryView() {
  const [tenant, setTenant] = useState("tenant-a");
  const [sku, setSku] = useState("SKU-100");
  const [item, setItem] = useState<InventoryItem | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load() {
    setError(null);
    try {
      setItem(await api.getInventory(tenant, sku));
    } catch (e: any) {
      setError(e.message);
      setItem(null);
    }
  }

  return (
    <div className="panel">
      <h2>Inventory</h2>
      <label>
        Tenant: <input value={tenant} onChange={(e) => setTenant(e.target.value)} />
      </label>{" "}
      <label>
        SKU: <input value={sku} onChange={(e) => setSku(e.target.value)} />
      </label>{" "}
      <button className="action" onClick={load}>
        Look up
      </button>
      {error && <div className="error-banner">{error}</div>}
      {item && (
        <table style={{ marginTop: "0.75rem" }}>
          <tbody>
            <tr>
              <th>SKU</th>
              <td>{item.SKU}</td>
            </tr>
            <tr>
              <th>On hand</th>
              <td>{item.OnHand}</td>
            </tr>
            <tr>
              <th>Reserved</th>
              <td>{item.Reserved}</td>
            </tr>
            <tr>
              <th>Available</th>
              <td>{item.OnHand - item.Reserved}</td>
            </tr>
          </tbody>
        </table>
      )}
    </div>
  );
}
