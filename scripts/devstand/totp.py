#!/usr/bin/env python3
"""Текущий TOTP-код (RFC 6238, SHA1, 30s) для dev-стенда.

Секрет по умолчанию — постоянный TOTP-секрет админа dev-стенда
(см. «Dev-стенд админки» в CLAUDE.md). Пример: python3 scripts/devstand/totp.py
"""
import base64
import hashlib
import hmac
import struct
import sys
import time

DEV_TOTP_SECRET = 'KUSECDEVTOTPSECRETKUSECDEVTOTP23'

key = base64.b32decode(sys.argv[1] if len(sys.argv) > 1 else DEV_TOTP_SECRET)
digest = hmac.new(key, struct.pack('>Q', int(time.time()) // 30), hashlib.sha1).digest()
offset = digest[-1] & 15
print('%06d' % ((struct.unpack('>I', digest[offset:offset + 4])[0] & 0x7FFFFFFF) % 1_000_000))
