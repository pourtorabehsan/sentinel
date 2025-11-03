#!/bin/bash
# Script to inspect Sentinel database

DB="sentinel.db"

if [ ! -f "$DB" ]; then
    echo "Database file not found: $DB"
    exit 1
fi

echo "=== GROUPS ==="
sqlite3 "$DB" "SELECT id, name, created_at FROM groups;"

echo ""
echo "=== CLUSTERS ==="
sqlite3 "$DB" "SELECT id, group_id, name, created_at FROM clusters;"

echo ""
echo "=== NODES ==="
sqlite3 "$DB" "SELECT id, cluster_id, host, port, role, source_node_id FROM nodes;"

echo ""
echo "=== NODE HEALTH ==="
sqlite3 "$DB" "SELECT node_id, reachable, replication_lag, io_thread_running, sql_thread_running FROM node_health;"

echo ""
echo "=== DOWNSTREAM RELATIONSHIPS ==="
sqlite3 "$DB" "SELECT node_id, downstream_node_id FROM node_downstream;"
