const BASE = (import.meta as any).env?.VITE_API_BASE ?? "/api/v1";

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`${res.status} ${res.statusText}: ${body}`);
  }
  if (res.status === 204) return undefined as unknown as T;
  return (await res.json()) as T;
}

export interface OrderItem {
  SKU: string;
  Quantity: number;
  UnitJPY: number;
}

export interface Order {
  ID: string;
  TenantID: string;
  CustomerID: string;
  Status: string;
  Items: OrderItem[];
  Version: number;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface InventoryItem {
  SKU: string;
  TenantID: string;
  OnHand: number;
  Reserved: number;
  UpdatedAt: string;
}

export interface AuditEvent {
  ID: string;
  TenantID: string;
  ActorID: string;
  Action: string;
  EntityID: string;
  Details: string;
  Timestamp: string;
}

export const api = {
  listOrders: (tenant: string) => req<Order[]>(`/orders?tenant=${encodeURIComponent(tenant)}`),
  getOrder: (id: string) => req<Order>(`/orders/${encodeURIComponent(id)}`),
  approveOrder: (id: string, actor: string) =>
    req<Order>(`/orders/${encodeURIComponent(id)}/approve`, { method: "POST", headers: { "X-Test-Actor": actor } }),
  cancelOrder: (id: string, actor: string) =>
    req<{ status: string }>(`/orders/${encodeURIComponent(id)}/cancel`, { method: "POST", headers: { "X-Test-Actor": actor } }),
  fulfilOrder: (id: string, actor: string) =>
    req<{ status: string }>(`/orders/${encodeURIComponent(id)}/fulfil`, { method: "POST", headers: { "X-Test-Actor": actor } }),
  adminBulkStatus: (orderIds: string[], status: string, actor: string) =>
    req<{ updated: number }>(`/admin/orders/bulk-status`, {
      method: "POST",
      headers: { "X-Test-Actor": actor },
      body: JSON.stringify({ order_ids: orderIds, status }),
    }),
  getInventory: (tenant: string, sku: string) =>
    req<InventoryItem>(`/inventory/${encodeURIComponent(tenant)}/${encodeURIComponent(sku)}`),
  getConfig: (tenant: string, key: string) => req<any>(`/configuration/${encodeURIComponent(tenant)}/${encodeURIComponent(key)}`),
  putConfig: (tenant: string, key: string, value: string) =>
    req<{ status: string }>(`/configuration/${encodeURIComponent(tenant)}/${encodeURIComponent(key)}`, {
      method: "PUT",
      body: JSON.stringify({ value }),
    }),
  listAudit: (tenant: string) => req<AuditEvent[]>(`/audit/${encodeURIComponent(tenant)}`),
  workerStatus: () => req<{ active: number; queued: number }>(`/workers/status`),
  configureSimulator: (dependency: string, tenant: string, body: { delay_ms: number; mode: string; fail_first_n: number }) =>
    req<{ status: string }>(`/admin/simulators/${dependency}/${encodeURIComponent(tenant)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
};