from django.urls import re_path

from . import consumers

websocket_urlpatterns = [
    re_path(r"ws/projects/(?P<project_id>[^/.]+)/pipelines/(?P<pipeline_id>[^/.]+)/jobs/(?P<job_id>[^/.]+)/logs/$", consumers.JobLogConsumer.as_asgi()),
]
