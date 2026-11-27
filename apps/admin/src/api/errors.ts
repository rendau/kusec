import { PASSWORD_MAX_LEN, PASSWORD_MIN_LEN } from '@/utils/password'

/**
 * API error type and the texts shown to the user.
 *
 * The backend never puts anything user-ready into `message` for plain errors:
 * a known code arrives as `{ code, message: code }`, an unexpected failure as
 * `{ code: "service_not_available", message: "internal error (request id: …)" }`.
 * Only errors raised with an explicit description carry it in `message`
 * (e.g. `invalid_request` — "keys not found in the cluster source: A, B").
 * So the text is built from `code` (+ `fields`), a description when there is
 * one, and the caller's fallback otherwise — never from a bare `message`.
 */

export class ApiError extends Error {
  readonly code: string
  readonly status: number
  readonly fields: Record<string, string>

  constructor(
    message: string,
    code: string,
    status: number,
    fields?: Record<string, string>,
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.status = status
    this.fields = fields ?? {}
  }
}

/** Code of an unexpected server failure (also used when no code arrived at all). */
export const UNKNOWN_ERROR_CODE = 'service_not_available'

/**
 * Texts for the codes of `internal/errs` (keep in sync with the Go constants).
 * A code missing here falls back to the caller's text with the code appended.
 */
const ERROR_TEXTS: Record<string, string> = {
  not_authorized: 'Your session has ended. Please sign in again.',
  no_permission: 'You do not have permission for this action',
  object_not_found: 'Not found — it may have been deleted',
  err_no_rows: 'Not found — it may have been deleted',
  invalid_request: 'Invalid request',
  incorrect_page_size: 'Invalid page size',
  id_required: 'ID is required',
  not_implemented: 'This action is not supported',
  invalid_config: 'The server is misconfigured',

  name_required: 'Name is required',
  username_required: 'Username is required',
  password_required: 'Password is required',
  password_too_short: `Password must be at least ${PASSWORD_MIN_LEN} characters`,
  password_too_long: `Password must be at most ${PASSWORD_MAX_LEN} bytes`,
  password_requires_upper: 'Password must contain an uppercase letter',
  password_requires_special: 'Password must contain a special character',
  current_password_invalid: 'Current password is incorrect',

  username_exists: 'This username is already taken',
  app_slug_exists: 'An application with this slug already exists',
  secret_slug_exists: 'A secret with this name already exists in the application',
  item_key_exists: 'An item with this key already exists in the secret',
  configmap_slug_exists:
    'A config map with this name already exists in the application',
  config_item_key_exists: 'An item with this key already exists in the config map',

  oauth_not_enabled: 'Sign-in for MCP clients is not enabled on this server',
  oauth_invalid_request: 'Invalid authorization request',

  element_active: 'The element has been re-activated',
  not_in_cluster: 'The service is not running inside a Kubernetes cluster',
  sync_in_progress: 'A sync is already in progress, try again in a moment',
}

function plural(count: number, unit: string): string {
  return `${count} ${unit}${count === 1 ? '' : 's'}`
}

/**
 * Wait time of a `too_many_attempts` error in whole seconds
 * (`fields.retry_after`, a string), or `null` when the backend did not say.
 */
export function retryAfterSeconds(error: unknown): number | null {
  if (!(error instanceof ApiError)) return null
  const seconds = Math.ceil(Number(error.fields.retry_after))
  return Number.isFinite(seconds) && seconds > 0 ? seconds : null
}

function tooManyAttemptsText(error: ApiError): string {
  const seconds = retryAfterSeconds(error)
  if (seconds === null) return 'Too many attempts. Try again later.'
  // Round minutes up: "in 2 minutes" must never end before the lock does.
  const wait =
    seconds < 60 ? plural(seconds, 'second') : plural(Math.ceil(seconds / 60), 'minute')
  return `Too many attempts. Try again in ${wait}.`
}

/**
 * Request id of an unexpected server failure, taken from
 * `internal error (request id: …)`; `null` when the message carries none.
 */
export function apiErrorRequestId(error: unknown): string | null {
  if (!(error instanceof ApiError)) return null
  return /request id:\s*([^\s)]+)/i.exec(error.message)?.[1] ?? null
}

/**
 * Description attached to the error by the backend, or '' when `message` is
 * just the code — bare, or at the end of a wrapped Go error chain as older
 * backends sent it ("Update: username_exists").
 */
function describedText(error: ApiError): string {
  const text = error.message.trim()
  if (!text || text.includes(error.code)) return ''
  return text.charAt(0).toUpperCase() + text.slice(1)
}

/**
 * Human-readable message for any error thrown by the API layer.
 * Use as the single place deciding between server and fallback text.
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  if (!(error instanceof ApiError)) return fallback

  if (error.code === 'too_many_attempts') return tooManyAttemptsText(error)

  // Unexpected failure: its message is deliberately impersonal, so show the
  // caller's text and keep only the request id — the handle to find it in logs.
  if (error.code === UNKNOWN_ERROR_CODE) {
    const requestId = apiErrorRequestId(error)
    return requestId ? `${fallback} (request id: ${requestId})` : fallback
  }

  return (
    describedText(error) || ERROR_TEXTS[error.code] || `${fallback} (${error.code})`
  )
}
