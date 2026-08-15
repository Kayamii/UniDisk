/**
 * Demo mode.
 *
 * When the app is built with VITE_DEMO=1 (the GitHub Pages build), there is no
 * Go backend to talk to. `isDemo` flips the API client over to an in-browser
 * backend that keeps all state in localStorage, so visitors can click through
 * the whole product — upload, browse, share, manage users — without a server
 * and without anything leaving their machine.
 *
 * In a normal build this constant is statically false, and every heavier demo
 * module is reached only through dynamic import, so bundlers drop the demo
 * backend and its seed data from the output entirely.
 */
export const isDemo = import.meta.env.VITE_DEMO === "1";

export { DEMO_EMAIL, DEMO_PASSWORD } from "./credentials";

/** resetDemo clears the visitor's local changes and restores the seeded state. */
export async function resetDemo(): Promise<void> {
  const { resetDemo: reset } = await import("./store");
  reset();
}
