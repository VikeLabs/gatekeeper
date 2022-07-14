#!/usr/bin/env bash
rm db.sqlite
sqlite3 db.sqlite < migrations/init.sql