#!/bin/bash
set -e
cd /opt/mygit/backend
echo ">>> Pulling..."
sudo git pull
echo ">>> Migrating..."
sudo DJANGO_SETTINGS_MODULE=config.settings.production ../venv/bin/python manage.py migrate --noinput
echo ">>> Building frontend..."
cd frontend && npm run build && sudo rm -rf /opt/mygit/static/assets /opt/mygit/static/index.html && sudo cp -r dist/* /opt/mygit/static/
echo ">>> Collecting static..."
cd /opt/mygit/backend && sudo DJANGO_SETTINGS_MODULE=config.settings.production ../venv/bin/python manage.py collectstatic --noinput -v0
echo ">>> Restarting..."
sudo systemctl restart nginx mygit-api mygit-git-http mygit-celery
echo ">>> Done. Visit http://$(hostname -I | awk '{print $1}'):8060"
