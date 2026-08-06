import os

from django.core.wsgi import get_wsgi_application

# Production entrypoint: never silently fall back to DEBUG=True local settings.
os.environ.setdefault("DJANGO_SETTINGS_MODULE", "config.settings.production")

application = get_wsgi_application()
