import { apiFetch } from './http'
import type { DashboardRep } from './types'

/**
 * REST client for the kusec `Dashboard` service
 * (see api/proto/kusec_v1/dashboard.proto). One call returns every number
 * the home page needs — counters and recently updated secrets.
 */

export function getDashboard(): Promise<DashboardRep> {
  return apiFetch<DashboardRep>('/dashboard')
}
