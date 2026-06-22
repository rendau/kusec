// Password complexity rules — mirror the backend validation in
// internal/usecase/usr/usecase.go (validatePassword) so the user gets instant
// feedback before the request is sent. Keep both sides in sync.

export const PASSWORD_MIN_LEN = 8
// bcrypt silently truncates anything past 72 bytes, so the backend caps length there.
export const PASSWORD_MAX_LEN = 72

/**
 * Returns an error message when `password` violates the complexity rules,
 * or null when it is acceptable.
 */
export function passwordComplexityError(password: string): string | null {
  if (password.length < PASSWORD_MIN_LEN) {
    return `Password must be at least ${PASSWORD_MIN_LEN} characters`
  }
  // Length in bytes — matches the backend's byte-based cap.
  if (new TextEncoder().encode(password).length > PASSWORD_MAX_LEN) {
    return `Password must be at most ${PASSWORD_MAX_LEN} bytes`
  }
  if (password === password.toLowerCase()) {
    return 'Password must contain an uppercase letter'
  }
  // A special char is anything that is not a letter, digit or whitespace.
  if (!/[^\p{L}\p{N}\s]/u.test(password)) {
    return 'Password must contain a special character'
  }
  return null
}
