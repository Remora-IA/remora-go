#!/bin/bash
cd "$(dirname "$0")"
echo "Sirviendo frontend en http://localhost:8085"
echo "API en http://localhost:8084"
echo ""
echo "Para detener: Ctrl+C"
python3 -m http.server 8085 2>/dev/null || python -m SimpleHTTPServer 8085
