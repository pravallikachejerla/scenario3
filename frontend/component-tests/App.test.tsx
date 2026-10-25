import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import App from "../src/App";

describe("App", () => {
  it("renders the primary navigation tabs", () => {
    render(<App />);
    expect(screen.getByRole("button", { name: "Orders" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Fulfilment Timeline" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Inventory" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Recovery Actions" })).toBeInTheDocument();
  });
});