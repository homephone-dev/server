FROM debian:bullseye-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    asterisk \
    gettext-base \
    && rm -rf /var/lib/apt/lists/*

COPY etc/asterisk/http.conf /etc/asterisk/http.conf
COPY etc/asterisk/pjsip.conf /etc/asterisk/pjsip.conf
COPY etc/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY etc/asterisk/ari.conf.template /etc/asterisk/ari.conf.template
COPY docker-entrypoint.sh /docker-entrypoint.sh

RUN chmod +x /docker-entrypoint.sh

EXPOSE 5060/udp 8088/tcp

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["asterisk", "-f", "-U", "asterisk", "-G", "asterisk"]
