@echo off

REM Script to run the test with a TOTP secret
REM This demonstrates how to run the test safely without hardcoded credentials

REM Example secret (replace with your actual secret when testing)
set TEST_TOTP_SECRET=RZCH2POUGIOAIDZJ2R2M4E62AIACDYVLF6WLDXG3KHWBCLZQL2ZA====

echo Running TOTP test with environment variable...
go run test_specific_secret.go 