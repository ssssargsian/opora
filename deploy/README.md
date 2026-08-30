# Opora production deployment

Production runs entirely in Docker Compose. Only Caddy publishes host ports `80/tcp` and `443/tcp`; PostgreSQL, MinIO, ClamAV, API, web, and ONLYOFFICE are reachable only over the Compose network.

The host must provide at least 2 vCPU, 4 GiB RAM, and 40 GiB free space for Docker data. The provisioner refuses an undersized host before creating secrets or application data; swap is added only as protection against short spikes and is not treated as RAM.

## Initial Ubuntu provisioning

DNS A records must point both names to the server:

```text
opora.cyberlineup.ru        155.212.223.107
office.opora.cyberlineup.ru 155.212.223.107
```

On a clean Ubuntu server:

```sh
git clone https://github.com/ssssargsian/opora.git /opt/opora
cd /opt/opora
chmod +x deploy/*.sh
./deploy/provision-ubuntu.sh
```

Provisioning installs only the required host tools, Docker Engine and the Compose plugin, configures UFW for SSH/HTTP/HTTPS, creates `/opt/opora/.env.production` with random secrets, builds images, applies migrations, and starts the stack. Re-running it preserves the environment file, volumes, and Caddy certificates.

## First administrator

After migrations, create the first organization and administrator. The generated initial password is shown once and is never written to an environment file:

```sh
cd /opt/opora
docker compose -f deploy/docker-compose.prod.yml --env-file .env.production \
  run --rm admin create \
  --email admin@example.org \
  --name "Администратор школы" \
  --organization "Название школы"
```

If the email already exists, the command fails without changing its password.

## Operations

Update without deleting persistent data:

```sh
cd /opt/opora && ./deploy/deploy.sh
```

Follow logs:

```sh
cd /opt/opora
docker compose -f deploy/docker-compose.prod.yml --env-file .env.production logs -f
```

Create a PostgreSQL backup:

```sh
cd /opt/opora && ./deploy/backup.sh
```

Stop containers without deleting volumes:

```sh
cd /opt/opora
docker compose -f deploy/docker-compose.prod.yml --env-file .env.production stop
```

Never run `docker compose down -v`. PostgreSQL dumps are only one part of recovery: configure encrypted off-server backup for both PostgreSQL and the private MinIO data before using real personal data.

## Networking

Production integration addresses are service names, never `localhost` or `host.docker.internal`:

- Browser: `https://opora.cyberlineup.ru`
- ONLYOFFICE browser URL: `https://office.opora.cyberlineup.ru`
- API to ONLYOFFICE: `http://onlyoffice`
- ONLYOFFICE to API: `http://api:8080`
- API to MinIO: `http://minio:9000`
- API to ClamAV: `clamav:3310`

The MinIO console is intentionally not public. Use a temporary SSH tunnel only for maintenance, then close it.

## SMTP invitations

Specialist accounts are activated through one-time invitation links. Add the following provider credentials to `.env.production` and run `./deploy/deploy.sh`:

```dotenv
SMTP_HOST=smtp.example.org
SMTP_PORT=587
SMTP_USERNAME=opora@example.org
SMTP_PASSWORD=provider-secret
SMTP_FROM_EMAIL=opora@example.org
SMTP_FROM_NAME=Опора
SMTP_TLS_MODE=starttls
```

Use `SMTP_TLS_MODE=tls` for implicit TLS (usually port 465). Passwords and invitation tokens must never be committed or logged. Without `SMTP_HOST`, account creation remains available but reports that delivery failed; an administrator can resend after SMTP is configured.

For a verified Yandex mailbox, use implicit TLS without committing the mailbox credentials:

```dotenv
SMTP_HOST=smtp.yandex.ru
SMTP_PORT=465
SMTP_USERNAME=verified-mailbox@example.ru
SMTP_PASSWORD=application-password
SMTP_FROM_EMAIL=verified-mailbox@example.ru
SMTP_FROM_NAME=Опора
SMTP_TLS_MODE=tls
```

Keep `.env.production` mode `600` after editing and recreate only the API container (`docker compose ... up -d --no-deps --force-recreate api`).
