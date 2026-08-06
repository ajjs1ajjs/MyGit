"""Celery tasks for asynchronous repository imports."""
import logging
import os
import shutil
import subprocess

from celery import shared_task

from apps.core.security import validate_public_http_url

logger = logging.getLogger("mygit")

CLONE_TIMEOUT = 600  # seconds


@shared_task(bind=True, max_retries=1, default_retry_delay=30)
def process_repository_import(self, job_id: str):
    from apps.administration.models import RepositoryImportJob

    try:
        job = RepositoryImportJob.objects.select_related("actor").get(id=job_id)
    except RepositoryImportJob.DoesNotExist:
        return

    if job.status in (RepositoryImportJob.Status.RUNNING, RepositoryImportJob.Status.SUCCESS):
        return

    job.status = RepositoryImportJob.Status.RUNNING
    job.error = ""
    job.save(update_fields=["status", "error"])

    try:
        _run_import(job)
        job.status = RepositoryImportJob.Status.SUCCESS
        job.save(update_fields=["status", "error"])
    except Exception as exc:
        job.status = RepositoryImportJob.Status.FAILED
        job.error = str(exc)[:2000]
        job.save(update_fields=["status", "error"])
        logger.warning("Repository import %s failed: %s", job_id, exc)


def _run_import(job):
    from apps.accounts.models import IntegrationToken
    from apps.git_service.backend import GitBackend
    from apps.git_service.hooks import install_hooks
    from apps.organizations.models import Group, GroupMember
    from apps.repositories.models import Repository, RepositoryAccess, validate_repository_path

    validate_repository_path(job.target_path)
    owner_path, repo_name = job.target_path.split("/", 1)

    provider = job.provider
    source = (job.source or "").strip()

    # --- Resolve the actual clone URL and credentials ---
    token = None
    if provider in ("github", "gitlab"):
        if not source or "://" in source:
            raise ValueError("source must be in 'owner/repo' format.")
        if provider == "github":
            host = "github.com"
            actual_url = f"https://github.com/{source.strip('/')}.git"
        else:
            host = "gitlab.com"
            actual_url = f"https://gitlab.com/{source.strip('/')}.git"
        integration = IntegrationToken.objects.filter(user=job.actor, provider=provider).first()
        if integration:
            token = integration.get_token()
        if token:
            auth_url = (
                f"https://oauth2:{token}@{host}/{source.strip('/')}.git"
                if provider == "gitlab"
                else f"https://{token}@{host}/{source.strip('/')}.git"
            )
        else:
            auth_url = actual_url
    else:  # gitea / custom: source is a full clone URL
        validate_public_http_url(source)
        actual_url = source
        auth_url = source

    # Reject private/internal hosts even for constructed URLs.
    validate_public_http_url(actual_url, allow_credentials=True)

    # --- Resolve the owner namespace ---
    if owner_path == job.actor.username:
        owner_type = "user"
        owner_id = job.actor.id
    else:
        group = Group.objects.filter(path=owner_path).first()
        if not group:
            raise ValueError(f"Target owner '{owner_path}' does not exist.")
        if not job.actor.is_superuser and not GroupMember.objects.filter(
            group=group, user=job.actor
        ).exists():
            raise ValueError("You do not have access to this group.")
        owner_type = "organization"
        owner_id = group.id

    if Repository.objects.filter(path=job.target_path).exists():
        raise ValueError("Repository already exists.")

    repo = Repository.objects.create(
        owner_type=owner_type,
        owner_id=owner_id,
        name=repo_name,
        path=job.target_path,
        description=f"Imported from {provider}",
        visibility="private",
        default_branch="main",
    )

    disk_path = None
    try:
        disk_path = repo.disk_path
        disk_path.parent.mkdir(parents=True, exist_ok=True)

        res = subprocess.run(
            ["git", "clone", "--bare", auth_url, str(disk_path)],
            capture_output=True,
            text=True,
            timeout=CLONE_TIMEOUT,
            env=os.environ.copy(),
        )
        if res.returncode != 0:
            err_msg = res.stderr
            if token:
                err_msg = err_msg.replace(token, "********")
            raise ValueError(f"Clone failed: {err_msg[:1000]}")

        # Strip any credentials git persisted in the remote config.
        subprocess.run(
            ["git", "-C", str(disk_path), "remote", "set-url", "origin", actual_url],
            capture_output=True,
            text=True,
            timeout=30,
        )

        install_hooks(disk_path)
        backend = GitBackend(disk_path)
        if backend.exists():
            repo.default_branch = backend.get_default_branch()
            repo.size_kb = GitBackend.get_repo_size_kb(disk_path)
        repo.save(update_fields=["default_branch", "size_kb"])

        RepositoryAccess.objects.create(
            user=job.actor, repository=repo, role=RepositoryAccess.Role.OWNER
        )
        job.repository = repo
        job.save(update_fields=["repository"])
    except Exception:
        if disk_path is not None and disk_path.exists():
            shutil.rmtree(disk_path)
        repo.delete()
        raise
