FROM golang:1.26

WORKDIR /opt/apiguard
COPY . .
RUN make build \
    && mkdir /var/opt/apiguard/status -p \
    && mkdir /var/opt/apiguard/internal -p

CMD ["./apiguard", "start", "conf.docker.json"]