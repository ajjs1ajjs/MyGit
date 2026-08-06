from pathlib import Path

import pytest
from django.contrib.auth import get_user_model
from django.core.exceptions import SuspiciousFileOperation, ValidationError
from rest_framework.test import APIClient
from rest_framework_simplejwt.tokens import RefreshToken

from apps.accounts.models import SSHKey
from apps.organizations.models import Group, GroupMember
from apps.repositories.models import Repository, RepositoryAccess

User = get_user_model()


def make_user(username: str, email: str | None = None, **extra_fields):
    return User.objects.create_user(
        email=email or f"{username}@example.com",
        username=username,
        password="StrongPass123!",
        **extra_fields,
    )


def make_repo(owner, name: str = "repo", visibility: str = Repository.Visibility.PRIVATE):
    return Repository.objects.create(
        owner_type="user",
        owner_id=owner.id,
        name=name,
        path=f"{owner.username}/{name}",
        visibility=visibility,
    )


@pytest.mark.django_db
def test_me_patch_cannot_escalate_to_superuser():
    user = make_user("alice")
    client = APIClient()
    client.force_authenticate(user=user)

    response = client.patch("/api/v1/users/me/", {"is_superuser": True}, format="json")

    assert response.status_code == 200
    user.refresh_from_db()
    assert user.is_superuser is False


@pytest.mark.django_db
def test_internal_check_access_denies_guest_push(settings, client):
    settings.MYGIT_INTERNAL_API_TOKEN = "test-internal-token"
    owner = make_user("owner")
    user = make_user("guest")
    repo = make_repo(owner)
    key = SSHKey.objects.create(
        user=user,
        title="laptop",
        public_key=(
            "ssh-ed25519 "
            "AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA test"
        ),
    )
    RepositoryAccess.objects.create(
        user=user,
        repository=repo,
        role=RepositoryAccess.Role.GUEST,
    )

    response = client.post(
        "/api/v1/internal/check_access",
        {"key_id": str(key.id), "repo_path": repo.path, "action": "push"},
        HTTP_AUTHORIZATION="Bearer test-internal-token",
    )

    assert response.status_code == 403


@pytest.mark.django_db
def test_repository_disk_path_rejects_traversal(settings, tmp_path):
    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")
    repo = Repository(name="repo", path="alice/../../outside", owner_id=make_user("alice").id)

    with pytest.raises(SuspiciousFileOperation):
        _ = repo.disk_path


@pytest.mark.django_db
def test_repository_custom_disk_path_resolves(settings, tmp_path):
    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")
    inside = tmp_path / "repos" / "custom-storage" / "my-repo.git"
    inside.parent.mkdir(parents=True, exist_ok=True)
    repo = Repository(
        name="my-repo",
        path="alice/my-repo",
        owner_id=make_user("alice").id,
        custom_disk_path=str(inside),
    )
    assert str(repo.disk_path).replace("\\", "/") == str(inside).replace("\\", "/")


@pytest.mark.django_db
def test_repository_custom_disk_path_rejects_outside_root(settings, tmp_path):
    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")
    repo = Repository(
        name="my-repo",
        path="alice/my-repo",
        owner_id=make_user("alice").id,
        custom_disk_path=str(tmp_path / "outside" / "my-repo.git"),
    )
    with pytest.raises(SuspiciousFileOperation):
        _ = repo.disk_path


@pytest.mark.django_db
def test_repository_clean_rejects_unsafe_name():
    repo = Repository(name="../outside", path="alice/../outside", owner_id=make_user("alice").id)

    with pytest.raises(ValidationError):
        repo.clean()


@pytest.mark.django_db
def test_browse_disk_and_create_folder(settings, tmp_path):
    # Use a path within the actual configured MYGIT_REPOS_ROOT
    allowed_root = Path(getattr(settings, "MYGIT_REPOS_ROOT", settings.BASE_DIR / "repos"))
    allowed_root.mkdir(parents=True, exist_ok=True)
    test_path = allowed_root / "test_browse"
    test_path.mkdir(exist_ok=True)

    user = make_user("testuser", is_superuser=True, is_staff=True)
    print(f"DEBUG: user={user.username}, is_superuser={user.is_superuser}, pk={user.pk}")
    client = APIClient()
    client.force_authenticate(user=user)

    response = client.get("/api/v1/projects/browse-disk/", {"path": str(test_path)})
    print(f"DEBUG: response={response.status_code}, data={response.data}")
    assert response.status_code == 200
    data = response.json()
    assert data["current_path"] is not None
    assert "directories" in data

    new_dir_name = "sub-folder"
    response = client.post("/api/v1/projects/create-disk-folder/", {
        "parent_path": str(test_path),
        "name": new_dir_name
    })
    assert response.status_code == 200
    assert response.json()["path"] == str(test_path / new_dir_name)
    assert (test_path / new_dir_name).exists()


@pytest.mark.django_db
def test_webhook_management_requires_maintainer_role():
    owner = make_user("owner")
    outsider = make_user("outsider")
    repo = make_repo(owner)
    client = APIClient()
    client.force_authenticate(user=outsider)

    response = client.get(f"/api/v1/projects/{repo.id}/hooks/")

    assert response.status_code == 403


@pytest.mark.django_db
def test_group_member_management_requires_group_maintainer():
    owner = make_user("groupowner")
    outsider = make_user("groupoutsider")
    target = make_user("target")
    group = Group.objects.create(name="Platform", path="platform")
    GroupMember.objects.create(group=group, user=owner, role=GroupMember.Role.OWNER)
    client = APIClient()
    client.force_authenticate(user=outsider)

    response = client.post(
        f"/api/v1/groups/{group.id}/members/",
        {"user": str(target.id), "role": GroupMember.Role.DEVELOPER},
        format="json",
    )

    assert response.status_code == 403


@pytest.mark.django_db
def test_guest_cannot_create_wiki_page():
    owner = make_user("wikiowner")
    guest = make_user("wikiguest")
    repo = make_repo(owner)
    RepositoryAccess.objects.create(
        user=guest,
        repository=repo,
        role=RepositoryAccess.Role.GUEST,
    )
    client = APIClient()
    client.force_authenticate(user=guest)

    response = client.post(
        f"/api/v1/projects/{repo.id}/wiki/",
        {"slug": "home", "title": "Home", "content": "Nope"},
        format="json",
    )

    assert response.status_code == 403


@pytest.mark.django_db
def test_public_repo_branch_create_requires_write_role():
    owner = make_user("publicowner")
    user = make_user("publicreader")
    repo = make_repo(owner, visibility=Repository.Visibility.PUBLIC)
    client = APIClient()
    client.force_authenticate(user=user)

    response = client.post(
        f"/api/v1/projects/{repo.id}/branches/",
        {"name": "feature"},
        format="json",
    )

    assert response.status_code == 403


@pytest.mark.django_db
def test_integration_token_management_requires_owner():
    from apps.accounts.models import IntegrationToken

    alice = make_user("alice")
    bob = make_user("bob")

    IntegrationToken.objects.create(user=alice, provider="github", token="ghp_alice12345")

    client = APIClient()
    client.force_authenticate(user=bob)

    # Bob trying to view Alice's integration tokens
    response = client.get(f"/api/v1/users/{alice.username}/integration-tokens/")
    assert response.status_code == 403

    # Bob trying to delete Alice's integration token
    response = client.delete(f"/api/v1/users/{alice.username}/integration-tokens/github/")
    assert response.status_code == 403

    # Alice managing her own tokens
    client.force_authenticate(user=alice)
    response = client.get(f"/api/v1/users/{alice.username}/integration-tokens/")
    assert response.status_code == 200
    assert len(response.data) == 1
    assert response.data[0]["provider"] == "github"
    assert response.data[0]["masked_token"] == "ghp_****2345"


@pytest.mark.django_db
def test_import_project_endpoint_creates_record(monkeypatch):
    import subprocess
    from unittest.mock import MagicMock

    from apps.repositories.models import Repository, RepositoryAccess

    owner = make_user("importer")
    client = APIClient()
    client.force_authenticate(user=owner)

    # Mock subprocess.run to simulate successful git clone
    mock_completed_process = MagicMock(returncode=0, stdout="", stderr="")
    mock_run = MagicMock(return_value=mock_completed_process)
    monkeypatch.setattr(subprocess, "run", mock_run)

    # Mock GitBackend size and active branch
    from apps.git_service.backend import GitBackend
    monkeypatch.setattr(GitBackend, "exists", lambda self: True)
    monkeypatch.setattr(GitBackend, "get_default_branch", lambda self: "main")
    monkeypatch.setattr(GitBackend, "get_repo_size_kb", lambda path: 42)

    # Mock install_hooks
    from apps.git_service import hooks
    monkeypatch.setattr(hooks, "install_hooks", lambda path: None)

    response = client.post(
        "/api/v1/projects/import/",
        {
            "provider": "github",
            "repo_name": "someowner/somerepo",
            "name": "myimportedrepo",
            "visibility": "private",
            "description": "Imported from GitHub",
        },
        format="json"
    )

    assert response.status_code == 201, response.data
    assert response.data["name"] == "myimportedrepo"
    assert response.data["visibility"] == "private"
    assert response.data["size_kb"] == 42

    # Verify DB record
    repo = Repository.objects.get(name="myimportedrepo")
    assert repo.path == "importer/myimportedrepo"
    assert repo.description == "Imported from GitHub"

    # Verify owner access granted
    access = RepositoryAccess.objects.get(repository=repo, user=owner)
    assert access.role == RepositoryAccess.Role.OWNER


@pytest.mark.django_db
def test_ci_update_log_requires_internal_token(settings):
    from apps.ci_cd.models import Job, Pipeline

    settings.MYGIT_INTERNAL_API_TOKEN = "runner-secret-token"
    owner = make_user("ciowner")
    repo = make_repo(owner)
    pipeline = Pipeline.objects.create(
        repository=repo, author=owner, ref="main", sha="a" * 40
    )
    job = Job.objects.create(pipeline=pipeline, name="test", stage="test")

    client = APIClient()
    client.force_authenticate(user=owner)

    # A regular authenticated user must NOT be able to write job logs / statuses.
    resp = client.post(
        f"/api/v1/projects/{repo.id}/pipelines/{pipeline.id}/jobs/{job.id}/update_log/",
        {"status": "success"},
        format="json",
    )
    assert resp.status_code == 403

    resp = client.post(
        f"/api/v1/projects/{repo.id}/pipelines/{pipeline.id}/jobs/{job.id}/claim/",
        {"runner_id": "r1"},
        format="json",
    )
    assert resp.status_code == 403

    # The runner (internal token) is allowed.
    resp = client.post(
        f"/api/v1/projects/{repo.id}/pipelines/{pipeline.id}/jobs/{job.id}/update_log/",
        {"log": "running tests\n"},
        format="json",
        HTTP_AUTHORIZATION="Bearer runner-secret-token",
    )
    assert resp.status_code == 200


@pytest.mark.django_db
def test_snippet_write_requires_author():
    from apps.snippets.models import Snippet

    alice = make_user("snipalice")
    bob = make_user("snipbob")
    snippet = Snippet.objects.create(
        author=alice, title="shared", code="print(1)", visibility="public"
    )

    client = APIClient()
    client.force_authenticate(user=bob)

    # Bob can read the public snippet but must not modify or delete it.
    resp = client.patch(
        f"/api/v1/snippets/{snippet.id}/", {"code": "print('hacked')"}, format="json"
    )
    assert resp.status_code == 404

    resp = client.delete(f"/api/v1/snippets/{snippet.id}/")
    assert resp.status_code == 404
    assert Snippet.objects.filter(id=snippet.id).exists()


@pytest.mark.django_db
def test_refresh_token_rotates_and_blacklists(settings):
    owner = make_user("refreshuser")

    refresh = RefreshToken.for_user(owner)
    client = APIClient()

    resp = client.post(
        "/api/v1/auth/refresh/", {"refresh": str(refresh)}, format="json"
    )
    assert resp.status_code == 200
    data = resp.json()
    assert data.get("access")
    assert data.get("refresh"), "ROTATE_REFRESH_TOKENS must return a new refresh token"

    # Replaying the old token must fail after rotation (blacklisted).
    resp2 = client.post(
        "/api/v1/auth/refresh/", {"refresh": str(refresh)}, format="json"
    )
    assert resp2.status_code == 401


@pytest.mark.django_db
def test_integration_token_encrypted_on_save():
    from apps.accounts.models import IntegrationToken

    alice = make_user("tokenowner")
    token = IntegrationToken.objects.create(
        user=alice, provider="github", token="ghp_plain_secret_123"
    )
    token.refresh_from_db()
    assert token.token.startswith("gAAAAA"), "token must be encrypted at rest"
    assert token.get_token() == "ghp_plain_secret_123"


@pytest.mark.django_db
def test_import_project_rejects_local_clone_url(settings):
    owner = make_user("importer2")
    client = APIClient()
    client.force_authenticate(user=owner)

    resp = client.post(
        "/api/v1/projects/import/",
        {
            "provider": "custom",
            "clone_url": "file:///etc/passwd",
            "name": "evilrepo",
        },
        format="json",
    )
    assert resp.status_code == 400


@pytest.mark.django_db
def test_repository_import_job_queues_and_processes(monkeypatch, settings, tmp_path):
    from unittest.mock import MagicMock

    from apps.administration.models import RepositoryImportJob
    from apps.repositories.models import Repository, RepositoryAccess

    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")

    owner = make_user("importerjob")
    client = APIClient()
    client.force_authenticate(user=owner)

    # Skip DNS lookups and real git clone in tests (eager Celery).
    import apps.administration.tasks as tasks

    monkeypatch.setattr(
        tasks, "validate_public_http_url", lambda url, allow_credentials=False: url
    )
    mock_completed = MagicMock(returncode=0, stdout="", stderr="")
    monkeypatch.setattr(tasks.subprocess, "run", lambda *a, **k: mock_completed)

    from apps.git_service import hooks
    from apps.git_service.backend import GitBackend

    monkeypatch.setattr(GitBackend, "exists", lambda self: True)
    monkeypatch.setattr(GitBackend, "get_default_branch", lambda self: "main")
    monkeypatch.setattr(GitBackend, "get_repo_size_kb", lambda path: 42)
    monkeypatch.setattr(hooks, "install_hooks", lambda path: None)

    resp = client.post(
        "/api/v1/repository-import-jobs/",
        {
            "provider": "github",
            "source": "example/repo",
            "target_path": "importerjob/imported",
        },
        format="json",
    )
    assert resp.status_code == 201, resp.data

    repo = Repository.objects.get(path="importerjob/imported")
    assert repo.owner_id == owner.id
    assert RepositoryAccess.objects.filter(
        repository=repo, user=owner, role=RepositoryAccess.Role.OWNER
    ).exists()

    job = RepositoryImportJob.objects.get(id=resp.json()["id"])
    assert job.status == RepositoryImportJob.Status.SUCCESS
    assert job.repository_id == repo.id


@pytest.mark.django_db
def test_repository_import_job_rejects_foreign_namespace(monkeypatch, settings, tmp_path):
    from unittest.mock import MagicMock

    from apps.administration.models import RepositoryImportJob
    from apps.repositories.models import Repository

    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")

    owner = make_user("importerjob2")
    make_user("otheruser")
    client = APIClient()
    client.force_authenticate(user=owner)

    import apps.administration.tasks as tasks

    monkeypatch.setattr(
        tasks, "validate_public_http_url", lambda url, allow_credentials=False: url
    )
    monkeypatch.setattr(
        tasks.subprocess, "run", lambda *a, **k: MagicMock(returncode=0, stdout="", stderr="")
    )

    resp = client.post(
        "/api/v1/repository-import-jobs/",
        {
            "provider": "github",
            "source": "example/repo",
            "target_path": "otheruser/imported",
        },
        format="json",
    )
    assert resp.status_code == 201, resp.data

    job = RepositoryImportJob.objects.get(id=resp.json()["id"])
    assert job.status == RepositoryImportJob.Status.FAILED
    assert not Repository.objects.filter(path="otheruser/imported").exists()


@pytest.mark.django_db
def test_repository_import_job_rejects_private_url(settings, tmp_path):
    settings.MYGIT_REPOS_ROOT = str(tmp_path / "repos")

    owner = make_user("importerjob3")
    client = APIClient()
    client.force_authenticate(user=owner)

    resp = client.post(
        "/api/v1/repository-import-jobs/",
        {
            "provider": "custom",
            "source": "http://127.0.0.1/internal.git",
            "target_path": "importerjob3/imported",
        },
        format="json",
    )
    assert resp.status_code == 400


@pytest.mark.django_db
def test_login_throttled():
    from django.core.cache import cache
    from rest_framework.throttling import SimpleRateThrottle

    cache.clear()
    rates = SimpleRateThrottle.THROTTLE_RATES
    original = rates.get("login")
    rates["login"] = "1/min"
    try:
        make_user("throttleuser")
        client = APIClient()

        resp1 = client.post(
            "/api/v1/auth/login/",
            {"login": "throttleuser", "password": "StrongPass123!"},
            format="json",
        )
        assert resp1.status_code == 200

        resp2 = client.post(
            "/api/v1/auth/login/",
            {"login": "throttleuser", "password": "StrongPass123!"},
            format="json",
        )
        assert resp2.status_code == 429
    finally:
        rates["login"] = original
        cache.clear()


@pytest.mark.django_db
def test_two_factor_provisioning_and_login():
    from apps.administration.totp import _code_for_counter

    user = make_user("2faprov")
    client = APIClient()
    client.force_authenticate(user=user)

    # Provision a TOTP device -> secret + otpauth_url returned
    resp = client.post(
        "/api/v1/two-factor-devices/",
        {"method": "totp", "name": "phone"},
        format="json",
    )
    assert resp.status_code == 201
    data = resp.json()
    secret = data["secret"]
    assert secret and data["otpauth_url"].startswith("otpauth://totp/")

    device_id = data["id"]
    # Generate a valid code for the current time window
    import time
    code = _code_for_counter(secret, int(time.time()) // 30)

    resp = client.post(
        f"/api/v1/two-factor-devices/{device_id}/verify/", {"code": code}, format="json"
    )
    assert resp.status_code == 200
    assert resp.json()["confirmed"] is True

    # Login now requires the OTP code
    anon = APIClient()
    resp = anon.post(
        "/api/v1/auth/login/",
        {"login": user.username, "password": "StrongPass123!"},
        format="json",
    )
    assert resp.status_code == 401

    code2 = _code_for_counter(secret, int(time.time()) // 30)
    resp = anon.post(
        "/api/v1/auth/login/",
        {"login": user.username, "password": "StrongPass123!", "otp": code2},
        format="json",
    )
    assert resp.status_code == 200
    assert resp.json().get("access")


@pytest.mark.django_db
def test_two_factor_required_at_login():
    from apps.administration.models import TwoFactorDevice
    from apps.administration.totp import generate_secret

    user = make_user("2fauser")
    secret = generate_secret()
    TwoFactorDevice.objects.create(
        user=user, method="totp", name="phone", secret=secret, confirmed=True
    )

    client = APIClient()
    # No OTP -> 401 with two-factor detail
    resp = client.post(
        "/api/v1/auth/login/",
        {"login": user.username, "password": "StrongPass123!"},
        format="json",
    )
    assert resp.status_code == 401

    # Wrong OTP -> 401
    resp = client.post(
        "/api/v1/auth/login/",
        {"login": user.username, "password": "StrongPass123!", "otp": "000000"},
        format="json",
    )
    assert resp.status_code == 401


