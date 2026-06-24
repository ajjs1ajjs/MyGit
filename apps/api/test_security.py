import pytest
from django.contrib.auth import get_user_model
from django.core.exceptions import SuspiciousFileOperation, ValidationError
from rest_framework.test import APIClient

from apps.accounts.models import SSHKey
from apps.organizations.models import Group, GroupMember
from apps.repositories.models import Repository, RepositoryAccess

User = get_user_model()


def make_user(username: str, email: str | None = None):
    return User.objects.create_user(
        email=email or f"{username}@example.com",
        username=username,
        password="StrongPass123!",
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
def test_repository_custom_disk_path_resolves():
    repo = Repository(
        name="my-repo",
        path="alice/my-repo",
        owner_id=make_user("alice").id,
        custom_disk_path="/var/git/custom-storage/my-repo.git"
    )
    assert str(repo.disk_path).replace("\\", "/").endswith("/var/git/custom-storage/my-repo.git")


@pytest.mark.django_db
def test_repository_clean_rejects_unsafe_name():
    repo = Repository(name="../outside", path="alice/../outside", owner_id=make_user("alice").id)

    with pytest.raises(ValidationError):
        repo.clean()


@pytest.mark.django_db
def test_browse_disk_and_create_folder(tmp_path):
    user = make_user("testuser")
    client = APIClient()
    client.force_authenticate(user=user)

    response = client.get("/api/v1/projects/browse-disk/", {"path": str(tmp_path)})
    assert response.status_code == 200
    data = response.json()
    assert data["current_path"] is not None
    assert "directories" in data

    new_dir_name = "sub-folder"
    response = client.post("/api/v1/projects/create-disk-folder/", {
        "parent_path": str(tmp_path),
        "name": new_dir_name
    })
    assert response.status_code == 200
    assert response.json()["path"] == str(tmp_path / new_dir_name)
    assert (tmp_path / new_dir_name).exists()


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


