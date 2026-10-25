import { useState } from "react";
import OrdersPage from "./pages/OrdersPage";
import FulfilmentTimeline from "./pages/FulfilmentTimeline";
import InventoryView from "./pages/InventoryView";
import BatchDashboard from "./pages/BatchDashboard";
import IncidentPanel from "./pages/IncidentPanel";
import ConfigurationView from "./pages/ConfigurationView";
import QueueMetrics from "./pages/QueueMetrics";
import RecoveryActions from "./pages/RecoveryActions";

const TABS = [
  { id: "orders", label: "Orders", Component: OrdersPage },
  { id: "fulfilment", label: "Fulfilment Timeline", Component: FulfilmentTimeline },
  { id: "inventory", label: "Inventory", Component: InventoryView },
  { id: "batch", label: "Batch Dashboard", Component: BatchDashboard },
  { id: "incidents", label: "Incident Panel", Component: IncidentPanel },
  { id: "configuration", label: "Configuration", Component: ConfigurationView },
  { id: "queue", label: "Queue / Retry Metrics", Component: QueueMetrics },
  { id: "recovery", label: "Recovery Actions", Component: RecoveryActions },
] as const;

export default function App() {
  const [tab, setTab] = useState<string>("orders");
  const active = TABS.find((t) => t.id === tab) ?? TABS[0];
  const Active = active.Component;

  return (
    <div className="app-shell">
      <header className="app-header">
        <h1>Order Management &amp; Fulfilment</h1>
        <span className="app-subtitle">Scenario 3 validation fixture</span>
      </header>
      <nav className="app-nav">
        {TABS.map((t) => (
          <button
            key={t.id}
            className={t.id === tab ? "nav-btn nav-btn-active" : "nav-btn"}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </nav>
      <main className="app-main">
        <Active />
      </main>
    </div>
  );
}