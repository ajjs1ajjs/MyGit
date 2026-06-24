from django.contrib import admin
from django.urls import include, path

urlpatterns = [
    path("api/v1/", include("apps.api.urls")),
    path("", include("apps.git_service.urls")),
    path("django-admin/", admin.site.urls),
    path("", include("django_prometheus.urls")),
]
