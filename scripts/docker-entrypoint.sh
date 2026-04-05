#!/bin/sh
chown -R appuser:appuser /data
exec gosu appuser "$@"
