#!/bin/sh

cd frontend
npm i
npm run build

cd ..
cd backend
echo "Backend"

mkdir -p web/html
rm -fr web/html/*
cp -R ../frontend/dist/* web/html/

go build -tags "with_quic,with_grpc,with_ech,with_utls,with_reality_server,with_acme,with_gvisor" -o ../sui main.go
