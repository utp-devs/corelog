#!/bin/bash
# Firewall base: todo entrante bloqueado salvo SSH, HTTP y HTTPS.
# IMPORTANTE: abrir SSH ANTES de habilitar ufw o se pierde el acceso.
set -euo pipefail

ufw default deny incoming
ufw default allow outgoing

ufw allow 22/tcp  comment 'SSH'
ufw allow 80/tcp  comment 'HTTP'
ufw allow 443/tcp comment 'HTTPS'

ufw --force enable
ufw status verbose
