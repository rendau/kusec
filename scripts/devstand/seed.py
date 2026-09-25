#!/usr/bin/env python3
"""Наполняет пустую БД dev-стенда: админ с постоянными кредами + демо-данные.

Админ заводится SQL-ом в контейнере kusec-dev-pg (bcrypt через pgcrypto,
2FA включена с постоянным секретом), остальное — через API запущенного
бэкенда. Порядок и параметры — «Dev-стенд админки» в CLAUDE.md.
"""
import base64
import json
import os
import subprocess
import urllib.request

API = os.environ.get('KUSEC_API', 'http://localhost:18080/api')
PG_CONTAINER = 'kusec-dev-pg'
USERNAME, PASSWORD = 'admin', 'admin12345'
HERE = os.path.dirname(os.path.abspath(__file__))


def call(method, path, body=None, token=None):
    headers = {'content-type': 'application/json'}
    if token:
        headers['authorization'] = 'Bearer ' + token
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(API + path, method=method, data=data, headers=headers)
    with urllib.request.urlopen(req) as resp:
        raw = resp.read()
        return json.loads(raw) if raw else {}


def seed_admin():
    sql = (
        'create extension if not exists pgcrypto;'
        'insert into usr (active, is_admin, name, username, password, totp_secret, totp_enabled) '
        f"values (true, true, 'Dev Admin', '{USERNAME}', crypt('{PASSWORD}', gen_salt('bf', 10)), "
        "'KUSECDEVTOTPSECRETKUSECDEVTOTP23', true);"
    )
    subprocess.run(
        ['docker', 'exec', PG_CONTAINER, 'psql', '-U', 'postgres', '-d', 'kusec_dev',
         '-v', 'ON_ERROR_STOP=1', '-c', sql],
        check=True,
    )


def login():
    code = subprocess.check_output(['python3', os.path.join(HERE, 'totp.py')]).decode().strip()
    return call('POST', '/usr/login', {'username': USERNAME, 'password': PASSWORD, 'totp_code': code})['jwt']


def seed_data(tok):
    app = call('POST', '/app', {
        'namespace': 'demo', 'name': 'Demo service', 'slug_name': 'demo',
        'description': 'Seed для проверки админки', 'active': True,
    }, tok)['id']

    def secret(slug, active=True):
        return call('POST', '/secret', {
            'app_id': app, 'slug_name': slug, 'description': 'secret ' + slug, 'active': active,
        }, tok)['id']

    def item(secret_id, key, value, fmt='text', active=True, enc='plain', file_name='', ctype=''):
        call('POST', '/item', {
            'secret_id': secret_id, 'key': key, 'value': value, 'value_format': fmt,
            'encoding': enc, 'file_name': file_name, 'content_type': ctype,
            'description': '', 'active': active,
        }, tok)

    main, db, legacy = secret('main'), secret('db'), secret('legacy', active=False)
    item(main, 'API_TOKEN', 'sk-live-7f3a9c2e1b8d4f60')
    item(main, 'SMTP_PASSWORD', 'p@ss w0rd with spaces', active=False)
    item(main, 'CONFIG_JSON', json.dumps({'retries': 3, 'endpoints': ['a', 'b'], 'timeout_ms': 1500}, indent=2), 'json')
    item(main, 'TLS_CERT', base64.b64encode(b'-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----\n').decode(),
         enc='base64', file_name='tls.crt', ctype='application/x-pem-file')
    item(db, 'PG_DSN', 'postgres://svc:secret@pg:5432/demo?sslmode=disable')
    item(db, 'PG_POOL', 'max: 20\nmin: 2\nidle_timeout: 30s\n', 'yaml')
    item(legacy, 'OLD_KEY', 'deprecated-value')

    def configmap(slug, active=True):
        return call('POST', '/configmap', {
            'app_id': app, 'slug_name': slug, 'description': 'configmap ' + slug, 'active': active,
        }, tok)['id']

    def config_item(configmap_id, key, value, fmt='text'):
        call('POST', '/config-item', {
            'configmap_id': configmap_id, 'key': key, 'value': value, 'value_format': fmt,
            'description': '', 'active': True,
        }, tok)

    cm_main, cm_flags = configmap('main'), configmap('feature-flags', active=False)
    config_item(cm_main, 'TZ', 'Asia/Almaty')
    config_item(cm_main, 'HTTP_PORT', '8080')
    config_item(cm_main, 'APP_YAML', 'server:\n  port: 8080\n  host: 0.0.0.0\n', 'yaml')
    config_item(cm_flags, 'FLAG_NEW_UI', 'true')
    return app


if __name__ == '__main__':
    seed_admin()
    print('app', seed_data(login()))
