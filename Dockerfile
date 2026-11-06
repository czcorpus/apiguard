FROM golang:1.26

ARG INSTALL_AIR=false
RUN if [ "$INSTALL_AIR" = "true" ]; then \
        curl -sSfL https://raw.githubusercontent.com/air-verse/air/master/install.sh | sh -s -- -b /usr/local/bin \
        && /usr/local/bin/air -v; \
    fi

WORKDIR /opt/apiguard
COPY . .
RUN git config --global --add safe.directory /opt/apiguard \
    && make build \
    && mkdir /var/opt/apiguard/status -p \
    && mkdir /var/opt/apiguard/internal -p

CMD ["./apiguard", "start", "conf.docker.json"]