#!/bin/sh
set -e

cd /var/www/html 2>/dev/null || cd /app/public

# wait for db
until curl --unix-socket /var/run/mysqld/mysqld.sock http://127.0.0.1 >/dev/null 2>&1; do
  rc=$?
  if [ "$rc" -eq 7 ]; then
    echo "⏳ Waiting for database socket..."
    sleep 2
  elif [ "$rc" -eq 1 ]; then
    echo "✅ Database socket is ready!"
    break
  else
    echo "⚠️ Unexpected curl return code: $rc"
    sleep 2
  fi
done

# check WordPress install
if ! wp core is-installed --allow-root; then
  echo "⚙️ Installing WordPress..."
  # [ ! -d "/wp-tmp/wp" ] && wp core download --allow-root --path="/wp-tmp/wp"
  # cp -a /wp-tmp/wp/* .
  wp core download --allow-root --path="."
  wp config create \
    --dbname="${WORDPRESS_DB_NAME:-wordpress}" \
    --dbuser="${WORDPRESS_DB_USER:-wpuser}" \
    --dbpass="${WORDPRESS_DB_PASSWORD:-wppass}" \
    --dbhost="${WORDPRESS_DB_HOST:-localhost:/var/run/mysqld/mysqld.sock}" \
    --dbprefix="${WORDPRESS_TABLE_PREFIX:-wp_}" \
    --dbcharset="${WORDPRESS_DB_CHARSET:-utf8mb4}" \
    --allow-root \
    --skip-check
  wp db create --allow-root || true
  wp core install \
    --url="${WP_URL:-http://127.0.0.1}" \
    --title="${WP_TITLE:-Test Site}" \
    --admin_user="${WP_ADMIN_USER:-admin}" \
    --admin_password="${WP_ADMIN_PASS:-adminpass}" \
    --admin_email="${WP_ADMIN_EMAIL:-admin@example.com}" \
    --skip-email \
    --allow-root
  wp rewrite structure '/%post_id%/%postname%/' --allow-root
  wp plugin install performance-lab --activate --allow-root

  # enable more details in server timing header
  #wp option update perflab_server_timing_settings --format=json '{"benchmarking_actions":[],"benchmarking_filters":[],"output_buffering":true}' --allow-root

  # setup some cache
  wp plugin install atec-cache-apcu --activate --allow-root
  wp option update atec_WPCA_settings --format=json '{"salt":"b8348898","o_cache":true,"o_admin":false,"o_stats":false,"p_cache":false,"p_admin":false,"p_debug":false}' --allow-root
  wp eval 'include( WP_PLUGIN_DIR . "/atec-cache-apcu/includes/atec-wpca-install-ocache.php" );use ATEC_WPCA\Install_OCache; Install_OCache::init(true);' --allow-root

  echo "✅ WordPress Installed with admin user"
fi

# import Theme Unit Test Data
POST_COUNT=$(wp post list --allow-root --format=count)
echo "POST_COUNT=${POST_COUNT}"
if [ "$POST_COUNT" -le 1 ]; then
  echo "📥 Importing Theme Unit Test Data..."
  # TEST_DATA="/wp-tmp/themeunittestdata.wordpress.xml"
  TEST_DATA="/tmp/themeunittestdata.wordpress.xml"
  [ ! -f "$TEST_DATA" ] && curl -L https://raw.githubusercontent.com/WordPress/theme-test-data/b9752e0533a5acbb876951a8cbb5bcc69a56474c/themeunittestdata.wordpress.xml \
    --output $TEST_DATA
  wp plugin install wordpress-importer --activate --allow-root
  wp import $TEST_DATA --authors=create --allow-root
  wp plugin deactivate wordpress-importer --allow-root
  echo "✅ Theme Unit Test Data Imported"
fi

exec "$@"
