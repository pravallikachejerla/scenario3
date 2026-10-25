import { useEffect, useState } from "react";
import { api, Order } from "../lib/api";

export default function OrdersPage() {
  const [tenant, setTenant] = useState("tenant-a");
  const [actor, setActor] = useState("agent-1");
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<Order | null>(null);

  async function load() {
    setError(null);
    try {
      setOrders(await api.listOrders(tenant));
    } catch (e: any) {
      setError(e.message);
    }
  }

  useEffect(() => {
    load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tenant]);

  async function act(fn: () => Promise<any>) {
    setError(null);
    try {
      await fn();
      await load();
    } catch (e: any) {
      setError(e.message);
    }
  }

  return (
    <div>
      <div className="panel">
        <h2>Order Search</h2>
        <label>
          Tenant:{" "}
          <input value={tenant} onChange={(e) => setTenant(e.target.value)} />
        </label>{" "}
        <label>
          Acting as:{" "}
          <input value={actor} onChange={(e) => setActor(e.target.value)} />
        </label>{" "}
        <button className="action" onClick={load}>
          Refresh
        </button>
        {error && <div className="error-banner">{error}</div>}
      </div>

      <div className="panel">
        <h2>Orders</h2>
        <table>
          <thead>
            <tr>
              <th>ID</th>
              <th>Customer</th>
              <th>Status</th>
              <th>Version</th>
              <th>Items</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {orders.map((o) => (
              <tr key={o.ID}>
                <td>
                  <a href="#" onClick={(e) => { e.preventDefault(); setSelected(o); }}>
                    {o.ID}
                  </a>
                </td>
                <td>{o.CustomerID}</td>
                <td>
                  <span className="status-pill">{o.Status}</span>
                </td>
                <td>{o.Version}</td>
                <td>{o.Items?.map((i) => `${i.SKU}×${i.Quantity}`).join(", ")}</td>
                <td>
                  <button className="action" onClick={() => act(() => api.approveOrder(o.ID, actor))}>
                    Approve
                  </button>
                  <button className="action" onClick={() => act(() => api.fulfilOrder(o.ID, actor))}>
                    Fulfil
                  </button>
                  <button className="action danger" onClick={() => act(() => api.cancelOrder(o.ID, actor))}>
                    Cancel
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {orders.length === 0 && <p className="hint">No orders for this tenant.</p>}
      </div>

      {selected && (
        <div className="panel">
          <h2>Order Detail — {selected.ID}</h2>
          <pre style={{ whiteSpace: "pre-wrap", fontSize: "0.8rem" }}>{JSON.stringify(selected, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}