from django.urls import re_path

from . import views

urlpatterns = [
    re_path(
        r"^(?P<owner>[^/]+)/(?P<repo_name>[^/]+)\.git/info/refs$",
        views.git_info_refs,
        name="git-info-refs",
    ),
    re_path(
        r"^(?P<owner>[^/]+)/(?P<repo_name>[^/]+)\.git/git-upload-pack$",
        views.git_rpc,
        name="git-upload-pack",
    ),
    re_path(
        r"^(?P<owner>[^/]+)/(?P<repo_name>[^/]+)\.git/git-receive-pack$",
        views.git_rpc,
        name="git-receive-pack",
    ),
]
