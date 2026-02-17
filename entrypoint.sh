#!/bin/sh

DB_PATH="${SUI_DB_FOLDER:-/app/db}/s-ui.db"
if [ -f "$DB_PATH" ]; then
	./sui migrate
fi

exec ./sui