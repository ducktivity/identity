import createOpenapiClient, { type Client } from 'openapi-fetch'
import type { paths } from './schema'

// Re-export the generated path/operation types so consumers can name request and response shapes (e.g. for typing a handler) without importing the schema module directly.
export type { paths, components, operations } from './schema'

/** A fully-typed openapi-fetch client bound to the identity service's contract. */
export type IdentityClient = Client<paths>

/**
 * Build a client for the shared identity service (id.ducktvt.com), the suite-wide sole
 * issuer of login codes and session tokens. Login is pre-auth and cross-origin (identity is
 * a different origin from every app backend), so this client adds no bearer-token or
 * 401-logout middleware — callers wire the returned token into their own app-backend client.
 *
 * @param baseUrl Identity's origin. Production: https://id.ducktvt.com. Local dev: the Go
 *   identity server, default http://localhost:8000 (the app backend runs on 8001).
 */
export function createClient(baseUrl: string): IdentityClient {
  return createOpenapiClient<paths>({ baseUrl })
}
