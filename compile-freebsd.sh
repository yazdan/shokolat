#!/bin/sh
export GOOS=freebsd
gb build
scp bin/shokolat-freebsd-amd64 root@192.168.3.5:~

