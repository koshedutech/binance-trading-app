@echo off
cd /d D:\Apps\binance-trading-bot
echo Starting Binance Trading Bot... > server.log 2>&1
go run . >> server.log 2>&1
