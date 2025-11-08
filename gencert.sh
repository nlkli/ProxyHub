#!/bin/bash

set -e

rm server.key 2> /dev/null
rm server.crt 2> /dev/null

openssl genrsa -out server.key 2048

read -p "Сертификат для (1)localhost или (2)внешний IP? [1/2]: " choice

if [ "$choice" = "1" ]; then
    openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650 \
        -subj "/C=RU/CN=localhost" \
        -addext "subjectAltName = DNS:localhost,IP:127.0.0.1"
    echo "localhost"
else
    ip=$(curl -s ifconfig.me/ip)
    openssl req -new -x509 -sha256 -key server.key -out server.crt -days 3650 \
        -subj "/C=RU/CN=$ip" \
        -addext "subjectAltName = IP:$ip,IP:127.0.0.1,DNS:localhost"
    echo "$ip"
fi

echo "server.key server.crt"
