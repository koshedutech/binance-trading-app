@echo off
cd /d D:\Apps\binance-trading-bot
del server.log 2>nul
binance-trading-bot.exe > server.log 2>&1
