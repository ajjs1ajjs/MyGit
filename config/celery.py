import os

from celery import Celery

os.environ.setdefault("DJANGO_SETTINGS_MODULE", "config.settings.local")

app = Celery("mygit")

app.config_from_object("django.conf:settings", namespace="CELERY")

app.autodiscover_tasks()


@app.task(bind=True, ignore_result=True)
def debug_task(self):
    import logging
    logging.getLogger("mygit").debug("Celery request: %r", self.request)
