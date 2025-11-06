FROM golang:1.23

WORKDIR /opt/apiguard
COPY . .
RUN make build \
    && mkdir /var/opt/apiguard/status -p

CMD ["./apiguard", "start", "conf.docker.json"]