#!/bin/bash

# get k6 here: https://github.com/grafana/k6/releases

export K6_WEB_DASHBOARD=true
export K6_WEB_DASHBOARD_EXPORT="${1:-output.html}"
export K6_WEB_DASHBOARD_PERIOD="1s"
export WP_URL="http://127.0.0.1"

./k6 run k6-bench.js
