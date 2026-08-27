import { useState } from "react";

export default function BatchDashboard() {
  const [batchSize, setBatchSize] = useState(200);

  return (
    <div className="panel">
      <h2>Batch Dashboard</h2>
      <p className="hint">
        Batch import and reprocessing jobs run through the backend batch processor
        (see docs/testing.md for how to exercise this against the private evaluator).
        This panel is a placeholder for the deterministic seed fixtures under
        <code> testdata/fixtures/</code> — small (200), medium (5,000) and large
        (20,000) synthetic order batches.
      </p>
      <label>
        Batch size:{" "}
        <input type="number" value={batchSize} onChange={(e) => setBatchSize(Number(e.target.value))} />
      </label>
      <p className="hint">
        Configure and dispatch batch jobs through <code>cmd/seed</code> and the
        worker process (see docs/operations.md).
      </p>
    </div>
  );
}
