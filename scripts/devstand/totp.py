#!/usr/bin/env python3
"""Текущий TOTP-код (RFC 6238, SHA1, 30s) — для проверки 2FA на dev-стенде.

Включить 2FA в профиле админки, скопировать ключ из «enter this key manually»
и получить код: python3 scripts/devstand/totp.py <ключ>
"""
import base64
import hashlib
import hmac
import struct
import sys
import time

if len(sys.argv) != 2:
    sys.exit('usage: totp.py <base32-секрет>')
key = base64.b32decode(sys.argv[1])
digest = hmac.new(key, struct.pack('>Q', int(time.time()) // 30), hashlib.sha1).digest()
offset = digest[-1] & 15
print('%06d' % ((struct.unpack('>I', digest[offset:offset + 4])[0] & 0x7FFFFFFF) % 1_000_000))
