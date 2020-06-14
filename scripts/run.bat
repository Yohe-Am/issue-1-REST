@echo off
pushd %~dp0
pushd ..
echo -- build started
go build -o ".build\issue1REST.exe" -i -v "cmd\server\main.go"
echo -- build completed
echo -- enter "k" to stop the server.
.build\issue1REST.exe
del .build\issue1REST.exe
popd
popd
pause