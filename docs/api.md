# API Reference (v1)

Base path: `/api/v1`. All responses are JSON. All endpoints require the
usual tenant/role context to be carried through in a production
deployment; the sandbox build accepts an `X-Test-Actor` header to
attribute audit events during testing.

| Method | Path | Description |
|---|---|---|
| GET | `/orders?tenant={id}` | List orders for a tenant. |
| GET | `/orders/{id}` | Get one order. |
| POST | `/orders/{id}/approve` | Approve a pending order. |
| POST | `/orders/{id}/cancel` | Cancel an order (workflow path). |
| POST | `/orders/{id}/fulfil` | Begin fulfilment (workflow path). |
| POST | `/admin/orders/bulk-status` | Administrative bulk status override. |
| GET | `/inventory/{tenant}/{sku}` | Get inventory levels for a SKU. |
| GET | `/configuration/{tenant}/{key}` | Get a configuration value. |
| PUT | `/configuration/{tenant}/{key}` | Update a configuration value. |
| GET | `/audit/{tenant}` | List audit events for a tenant. |
| GET | `/workers/status` | Worker pool active/queued counts. |
| POST | `/admin/simulators/{dependency}/{tenant}` | Sandbox-only dependency failure-injection control (see App.SandboxMode). |

Top-level (unversioned) endpoints: `GET /healthz`, `GET /readyz`,
`GET /version`, `GET /metrics`.

## Example

```
GET /api/v1/orders/ord-1
200 OK
{
  "ID": "ord-1",
  "TenantID": "tenant-a",
  "CustomerID": "cust-1",
  "Status": "pending",
  "Items": [{"SKU": "SKU-100", "Quantity": 2, "UnitJPY": 3500}],
  "Version": 0,
  "CreatedAt": "...",
  "UpdatedAt": "..."
}
```
