FROM postgres:16-alpine

# Self-signed dev-only TLS cert so the backend can connect with
# sslmode=require. NOT for production use — replace with a real cert issued
# by your own CA or a proper ACME flow when deploying for real.
RUN apk add --no-cache openssl \
    && openssl req -new -x509 -days 3650 -nodes -text \
       -out /var/lib/postgresql/server.crt \
       -keyout /var/lib/postgresql/server.key \
       -subj "/CN=postgres" \
    && chmod 600 /var/lib/postgresql/server.key \
    && chown postgres:postgres /var/lib/postgresql/server.crt /var/lib/postgresql/server.key

CMD ["postgres", "-c", "ssl=on", "-c", "ssl_cert_file=/var/lib/postgresql/server.crt", "-c", "ssl_key_file=/var/lib/postgresql/server.key"]
