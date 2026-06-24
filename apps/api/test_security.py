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
def test_repository_clean_rejects_unsafe_name():
    repo = Repository(name="../outside", path="alice/../outside", owner_id=make_user("alice").id)

    with pytest.raises(ValidationError):
        repo.clean()


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
