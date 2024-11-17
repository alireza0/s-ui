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

go build -o ../sui main.go
