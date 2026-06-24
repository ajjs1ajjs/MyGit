import pytest


@pytest.fixture(autouse=True)
def use_sqlite_db(settings):
    settings.DATABASES = {
        "default": {
            "ENGINE": "django.db.backends.sqlite3",
            "NAME": ":memory:",
        }
    }


@pytest.fixture(autouse=True)
def disable_celery(settings):
    settings.CELERY_TASK_ALWAYS_EAGER = True
    settings.CELERY_BROKER_URL = "memory://"
