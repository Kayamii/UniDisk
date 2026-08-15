// Demo sign-in credentials, kept in their own module so the login screen can
// display them without statically importing the whole demo backend (which would
// pull it into the production bundle).

/** The seeded admin, prefilled on the demo login form. */
export const DEMO_EMAIL = "admin@unidisk.local";

/** Any seeded user signs in with this password. */
export const DEMO_PASSWORD = "demo";
