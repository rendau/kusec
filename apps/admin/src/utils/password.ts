// Password complexity rules — mirror the backend validation in
// internal/usecase/usr/usecase.go (validatePassword) so the user gets instant
// feedback before the request is sent. Keep both sides in sync.

import { randomInt } from './random'

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

export const GENERATED_PASSWORD_LEN = 16

// Alphabets for generated passwords. Look-alike characters (0/O, 1/l/I) are
// left out because the password is retyped by a human, and the specials avoid
// the ones messengers treat as markup or links (* _ ~ ` @) — the password is
// sent in a chat message and must survive it verbatim.
const PASSWORD_CHAR_CLASSES = [
  'abcdefghijkmnpqrstuvwxyz',
  'ABCDEFGHJKLMNPQRSTUVWXYZ',
  '23456789',
  '!#$%&+=?',
]

/**
 * Generates a random password that satisfies `passwordComplexityError`:
 * at least one character of every class, the rest drawn from the whole
 * alphabet, then shuffled so the guaranteed characters are not positional.
 */
export function generatePassword(length = GENERATED_PASSWORD_LEN): string {
  const alphabet = PASSWORD_CHAR_CLASSES.join('')
  const pick = (chars: string): string => chars[randomInt(chars.length)]!

  const chars = PASSWORD_CHAR_CLASSES.map(pick)
  while (chars.length < length) chars.push(pick(alphabet))

  // Fisher–Yates shuffle.
  for (let i = chars.length - 1; i > 0; i--) {
    const j = randomInt(i + 1)
    ;[chars[i], chars[j]] = [chars[j]!, chars[i]!]
  }
  return chars.join('')
}
